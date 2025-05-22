#!/bin/sh

set -e

_is_git_dir() {
	if [ -d "$1/.git" ]; then
		return 0
	else
		return 1
	fi
}

_pull() {
	dir="$1"
	cd "$dir" > /dev/null 2>&1

	# check if there is a remote
	if [ -z "$(git remote -v)" ]; then
		echo "No remote found"
		return 0
	fi

	# check if the remote is reachable
	if ! git remote show origin > /dev/null 2>&1; then
		echo "Remote not reachable"
		return 0
	fi

	# skip if not in main or master branch
    if [ "$(git rev-parse --abbrev-ref HEAD)" != "main" ] && [ "$(git rev-parse --abbrev-ref HEAD)" != "master" ]; then
    	echo "Not on main or master branch: $(git rev-parse --abbrev-ref HEAD)"
    	return 0
    fi

    # skip if there are local changes
    if [ -n "$(git status --porcelain)" ]; then
    	echo "Local changes found"
    	return 0
    fi

    git fetch --all
    git pull
	cd - > /dev/null 2>&1
}

# Main function to walk through directories and update Git repos
_walk_and_pull() {
	for dir in "$1"/*; do
		if [ -d "$dir" ]; then
			if _is_git_dir "$dir"; then
				echo "Pulling in: $dir"
				_pull "$dir"
			else
				_walk_and_pull "$dir"
			fi
		fi
	done
}

_git_pull() {
	search_dir=$1
	if [ -z "$search_dir" ]; then
		search_dir="."
	fi
	_walk_and_pull "$search_dir"
	echo "Git pull script completed."
}

case $0 in
*bash) ;;
*) _git_pull "$@" ;;
esac

unset _git_pull
unset _walk_and_pull
unset _pull
unset _is_git_dir
