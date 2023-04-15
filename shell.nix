{ systemPkgs ? import <nixpkgs> {} }:

let lib = systemPkgs.lib;
	overlay = self: super: {
		go = super.go_1_19;
	};
	pkgs = import (systemPkgs.fetchFromGitHub {
		owner = "NixOS";
		repo  = "nixpkgs";
		rev   = "27ccd29078f974ddbdd7edc8e38c8c8ae003c877";
		hash  = "sha256:1lsjmwbs3nfmknnvqiqbhh103qzxyy3z1950vqmzgn5m0zx7048h";
	}) {
		overlays = [ overlay ];
	};

	sqlc = pkgs.buildGoModule rec {
		name = "sqlc";
		version = "1.17.2";

		src = pkgs.fetchFromGitHub {
			owner  = "kyleconroy";
			repo   = "sqlc";
			rev    = "v${version}";
			sha256 = "0kq9f605pwjvs2azc5bq6k6iji54yp1dmsnrb7lfj2rvilb4hiyz";
		};

		proxyVendor = true;
		vendorSha256 = "0ih9siizn6nkvm4wi0474wxy323hklkhmdl52pql0qzqanmri4yb";

		# Skip fuzz tests.
		doCheck = false;
	};

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
