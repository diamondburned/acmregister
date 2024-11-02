{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-22.11";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:

    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        lib = pkgs.lib;

        sql-formatter = pkgs.writeShellScriptBin "sql-formatter" ''
          set -e

          [[ "$1" ]] && stdin=$(< "$1") || stdin=$(cat)
          language=$(command grep -oP '^-- Language: \K.*$' <<< "$stdin")

          config_for_lang() {
          	cat<<-EOF
          	{
          	  "expressionWidth": 50,
          	  "keywordCase": "upper",
          	  "language": "$1",
          	  "tabulateAlias": false,
          	  "useTabs": true
          	}
          	EOF
          }

          ${pkgs.nodePackages.sql-formatter}/bin/sql-formatter \
          	--config <(config_for_lang "$language") <<< "$stdin"
        '';
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gotools # std go tools
            go-tools # dominikh
            gopls
            colordiff
            sqlc
            sql-formatter
          ];
        };
      }
    );
}
