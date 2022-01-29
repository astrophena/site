#!/usr/bin/env bash

# Install dotfiles.

if test -z "$BASH_VERSION"; then
	echo "Please run this script using bash, not sh or any other shell." >&2
	exit 1
fi

set -euo pipefail

# We wrap the entire script in a big function which we only call at
# the very end, in order to protect against the possibility of the
# connection dying mid-script.
_() {
	[[ -z "${DEBUG:-}" ]] || set -x

	no_color='\033[0m'

	info() {
		local yellow_color='\033[0;33m'
		echo -e "${yellow_color}"$@"${no_color}"
	}

	error() {
		local red_color='\033[0;31m'
		echo -e "${red_color}"$@"${no_color}"
		exit 1
	}

	require_command() {
		command -v "$1" &>/dev/null || {
			error ""$2" is required. Install it and try again."
		}
	}

	[[ -d "$HOME/src/dotfiles" ]] &&
		error "dotfiles are already installed. Run u to update them."

	require_command "rcup" "rcm"
	require_command "git" "Git"
	require_command "vim" "Vim"

	info "==> Cloning Git repository..."
	git clone -q https://github.com/astrophena/dotfiles "$HOME/src/dotfiles"

	info "==> Installing dotfiles..."
	RCRC="$HOME/src/dotfiles/rcrc" rcup -f

	info "==> Installing Vim plugins..."
	vim -es -u ~/.vim/vimrc -i NONE -c "PlugInstall" -c "qa"
}

# Now that we know the whole script has downloaded, run it.
_ "$0" "$@"
