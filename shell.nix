{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  name = "site";
  packages = with pkgs; [ git go_1_17 goimports nodejs ];
  shellHook = ''
    export PS1="$PS1(site) "
    if [[ ! -d node_modules ]]; then
      echo "==> Installing Node.js dependencies..."
      npm install
    fi
  '';
}
