{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = with pkgs; [
    # Go development
    go
    gopls

    # Build tools
    gnumake
    git

    # SSH for deployment testing
    openssh
  ];

  shellHook = ''
    echo "Juniper Server - Development Environment"
    echo "========================================="
    echo ""
    echo "Commands:"
    echo "  make build        Build juniper-host binary"
    echo "  make test         Run tests"
    echo "  make install      Install to /usr/local/bin"
    echo "  make release-local Build for all platforms"
    echo ""
  '';
}
