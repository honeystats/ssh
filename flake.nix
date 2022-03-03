{
  description = "ssh";

  outputs = { self, nixpkgs }: let
    pkgs = import nixpkgs {
      system = "x86_64-linux";
    };
    gow = pkgs.writeShellScriptBin "gow" ''
      ${pkgs.findutils}/bin/find . -name "*.go" | ${pkgs.entr}/bin/entr -r go "$@"
    '';
    conn = pkgs.writeShellScriptBin "conn" ''
      while true; do
        ssh-keygen -f "/home/$USER/.ssh/known_hosts" -R "[localhost]:2222"
        ${pkgs.sshpass}/bin/sshpass -p 'password' ssh -p 2222 localhost -o stricthostkeychecking=no
        sleep 2
      done
    '';
  in {
    devShell.x86_64-linux = pkgs.mkShell {
      name = "ssh";
      buildInputs = with pkgs; [
        go
        gow
        gopls
        conn
      ];
    };
  };
}
