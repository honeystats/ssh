{
  description = "ssh";

  outputs = { self, nixpkgs }: let
    pkgs = import nixpkgs {
      system = "x86_64-linux";
    };
  in {
    devShell.x86_64-linux = pkgs.mkShell {
      name = "ssh";
      buildInputs = with pkgs; [
        go
        gopls
      ];
    };
  };
}
