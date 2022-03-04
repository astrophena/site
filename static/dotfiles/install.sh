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
	yellow_color='\033[0;33m'

	info() {
		echo -e "${yellow_color}"$@"${no_color}"
	}

	DOTFILES="$HOME/src/dotfiles"
	[[ "$CODESPACES" == "true" ]] && {
		# GitHub Codespaces clones dotfiles there.
		# See https://docs.github.com/en/codespaces/troubleshooting/troubleshooting-dotfiles-for-codespaces.
		DOTFILES="/workspaces/.codespaces/.persistedshare/dotfiles"
	}
	export PATH="$DOTFILES/vendor/rcm/bin:$PATH"

	[[ ! -d "$DOTFILES" ]] && {
		info "==> Cloning Git repository..."
		git clone -q https://github.com/astrophena/dotfiles "$DOTFILES"
	}

	info "==> Installing dotfiles..."
	RCRC="$DOTFILES/rcrc" rcup -f

	command -v vim &>/dev/null && {
		info "==> Installing Vim plugins..."
		vim -es -u ~/.vim/vimrc -i NONE -c "PlugInstall" -c "qa"
	}
}

# Now that we know the whole script has downloaded, run it.
_ "$0" "$@"
