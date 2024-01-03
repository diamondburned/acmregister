let
	pkgsOverlay = self: super: {
		go = super.go_1_19;
	};
in

{
	systemPkgs ? import <nixpkgs> {
		overlays = [ pkgsOverlay ];
	}
}:

let
	pkgs = systemPkgs;
	lib = systemPkgs.lib;

	sql-formatter = systemPkgs.writeShellScriptBin "sql-formatter" ''
		set -e

		[[ "$1" ]] && stdin=$(< "$1") || stdin=$(cat)
		language=$(command grep -oP '^-- Language: \K.*$' <<< "$stdin")

		config_for_lang() {
			cat<<EOF
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

in pkgs.mkShell {
	buildInputs = with pkgs; [
		go
		gotools  # std go tools
		go-tools # dominikh
		gopls
		colordiff
		sqlc
		sql-formatter
	];
}
