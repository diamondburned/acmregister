{ systemPkgs ? import <nixpkgs> {} }:

let lib = systemPkgs.lib;
	pkgs =
		if (lib.versionAtLeast systemPkgs.go.version "1.19")
		then systemPkgs
		else import (systemPkgs.fetchFromGitHub {
			owner = "NixOS";
			repo  = "nixpkgs";
			rev   = "e105167e98817ba9fe079c6c3c544c6ef188e276";
			hash  = "sha256:1274abx6ibdlavvm43a398rkb3fnhr1s5n7fjiv9l9zzpjgrdyvq";
		}) {};

	sqlc = pkgs.buildGoModule {
		name = "sqlc";
		version = "1.15.0";

		src = pkgs.fetchFromGitHub {
			owner  = "kyleconroy";
			repo   = "sqlc";
			rev    = "v1.15.0";
			sha256 = "11iinay0din8rjd20sbjipqvsvarw6r0sd5vgfqjq5bcx41vkxji";
		};

		proxyVendor = true;
		vendorSha256 = "0ycvjn6yfdijr2avhw16ayn0s1lcmgvxd901l0129ks2xbhlbar9";

		# Skip fuzz tests.
		doCheck = false;
	};

	goose = pkgs.buildGoModule {
		name = "goose";
		version = "3.5.3";

		src = pkgs.fetchFromGitHub {
			owner  = "pressly";
			repo   = "goose";
			rev    = "5f1f43cfb2ba11d901b1ea2f28c88bf2577985cb";
			sha256 = "13hcbn4v78142brqjcjg1q297p4hs28n25y1fkb9i25l5k2bwk7f";
		};

		vendorSha256 = "1yng6dlmr4j8cq2f43jg5nvcaaik4n51y79p5zmqwdzzmpl8jgrv";
		subPackages = [ "cmd/goose" ];
	};

	sql-formatter = systemPkgs.writeShellScriptBin "sql-formatter" ''
		${pkgs.nodePackages.sql-formatter}/bin/sql-formatter --config ${./sql-formatter.json}
	'';

in pkgs.mkShell {
	buildInputs = with pkgs; [
		go
		gotools
		gopls
		goose
		sqlc
		sql-formatter
	];
}
