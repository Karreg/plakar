#!/bin/bash

publish_tui_tag() (
	set -euo pipefail

	if [[ $# -ne 1 || -z $1 ]]; then
		printf 'usage: publish_tui_tag <tag>\n' >&2
		return 2
	fi

	local tag=$1
	local repository

	if ! git check-ref-format "refs/tags/$tag" >/dev/null; then
		printf "publish_tui_tag: invalid tag: %s\n" "$tag" >&2
		return 2
	fi
	if ! repository=$(git rev-parse --show-toplevel 2>/dev/null); then
		printf 'publish_tui_tag: not in a Git repository\n' >&2
		return 1
	fi
	cd "$repository"

	if ! git remote get-url upstream >/dev/null 2>&1; then
		printf "publish_tui_tag: remote 'upstream' does not exist\n" >&2
		return 1
	fi
	if ! git remote get-url origin >/dev/null 2>&1; then
		printf "publish_tui_tag: remote 'origin' does not exist\n" >&2
		return 1
	fi
	if ! git rev-parse --verify --quiet refs/heads/main^{commit} >/dev/null; then
		printf "publish_tui_tag: local branch 'main' does not exist\n" >&2
		return 1
	fi
	if ! git rev-parse --verify --quiet refs/heads/plakar_tui^{commit} >/dev/null; then
		printf "publish_tui_tag: local branch 'plakar_tui' does not exist\n" >&2
		return 1
	fi

	local upstream_tag
	if ! upstream_tag=$(git ls-remote --refs upstream "refs/tags/$tag"); then
		printf "publish_tui_tag: could not query upstream\n" >&2
		return 1
	fi
	if [[ -z $upstream_tag ]]; then
		printf "publish_tui_tag: tag '%s' does not exist on upstream\n" "$tag" >&2
		return 1
	fi

	local origin_tag
	if ! origin_tag=$(git ls-remote --refs origin "refs/tags/$tag"); then
		printf "publish_tui_tag: could not query origin\n" >&2
		return 1
	fi
	if [[ -n $origin_tag ]]; then
		printf "publish_tui_tag: tag '%s' already exists on origin\n" "$tag" >&2
		return 1
	fi

	local temporary_directory
	temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/publish-tui-tag.XXXXXXXX")
	local worktree=$temporary_directory/worktree
	local worktree_added=false
	local source_ref="refs/publish-tui-tag/$$-$RANDOM/source"
	local temporary_tag="publish-tui-tag-$$-$RANDOM"
	local temporary_tag_ref="refs/tags/$temporary_tag"

	cleanup_publish_tui_tag() {
		local status=$?

		if [[ $worktree_added == true ]]; then
			git worktree remove --force "$worktree" >/dev/null 2>&1 || true
		fi
		git update-ref -d "$source_ref" >/dev/null 2>&1 || true
		git update-ref -d "$temporary_tag_ref" >/dev/null 2>&1 || true
		rm -rf "$temporary_directory"
		exit "$status"
	}
	trap cleanup_publish_tui_tag EXIT

	if ! git fetch --no-tags upstream "+refs/tags/$tag:$source_ref"; then
		printf "publish_tui_tag: could not fetch upstream tag '%s'\n" "$tag" >&2
		return 1
	fi

	local upstream_commit
	upstream_commit=$(git rev-parse --verify "$source_ref^{}")
	if ! git cat-file -e "$upstream_commit^{commit}" 2>/dev/null; then
		printf "publish_tui_tag: upstream tag '%s' does not point to a commit\n" "$tag" >&2
		return 1
	fi

	local patch_file=$temporary_directory/plakar-tui.patch
	git diff --binary refs/heads/main...refs/heads/plakar_tui >"$patch_file"
	if [[ ! -s $patch_file ]]; then
		printf 'publish_tui_tag: main...plakar_tui produces an empty patch\n' >&2
		return 1
	fi

	git worktree add --detach "$worktree" "$upstream_commit" >/dev/null
	worktree_added=true
	if ! git -C "$worktree" apply --index --3way "$patch_file"; then
		printf "publish_tui_tag: patch does not apply to upstream tag '%s'\n" "$tag" >&2
		return 1
	fi
	if ! (cd "$worktree" && go mod tidy); then
		printf "publish_tui_tag: could not update Go module metadata\n" >&2
		return 1
	fi
	git -C "$worktree" add -- go.mod go.sum
	if git -C "$worktree" diff --cached --quiet; then
		printf 'publish_tui_tag: applying the patch produced no changes\n' >&2
		return 1
	fi

	git -C "$worktree" commit -m "Apply plakar_tui changes to $tag"
	local result_commit
	result_commit=$(git -C "$worktree" rev-parse HEAD)
	git tag -a "$temporary_tag" "$result_commit" -m "Apply plakar_tui changes to $tag"
	git push origin "$temporary_tag_ref:refs/tags/$tag"

	printf 'publish_tui_tag: published %s at %s\n' "$tag" "$result_commit"
)

