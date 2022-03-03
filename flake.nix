{
  description = "ssh";

  outputs = { self, nixpkgs }: let
    pkgs = import nixpkgs {
      system = "x86_64-linux";
    };
    gow = pkgs.writeShellScriptBin "gow" ''
      ${pkgs.findutils}/bin/find . -name "*.go" | ${pkgs.entr}/bin/entr -r go "$@"
    '';
  in {
    devShell.x86_64-linux = pkgs.mkShell {
      name = "ssh";
      buildInputs = with pkgs; [
        go
        gow
        gopls
      ];
    };
  };
}
