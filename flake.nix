{
  description = "Development shell for the Baseten Terraform provider";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { nixpkgs, ... }:
    let
      systems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-darwin"
        "x86_64-linux"
      ];

      forEachSystem = nixpkgs.lib.genAttrs systems;
    in
    {
      devShells = forEachSystem (system:
        let
          pkgs = import nixpkgs {
            inherit system;
            config.allowUnfreePredicate = package:
              builtins.elem (nixpkgs.lib.getName package) [
                "terraform"
              ];
          };
        in
        {
          default = pkgs.mkShell {
            packages = with pkgs; [
              curl
              delve
              git
              go
              golangci-lint
              gopls
              gotools
              goreleaser
              jq
              terraform
              terraform-plugin-docs
              uv
            ];

            shellHook = ''
              export GOPATH="$PWD/.go"
              export GOBIN="$GOPATH/bin"
              export PATH="$GOBIN:$PATH"

              echo "Baseten Terraform provider dev shell"
              echo "Go:        $(go version)"
              echo "Terraform: $(terraform version -json | jq -r .terraform_version)"
              echo "Baseten CLI and Truss can be run with: uvx baseten, uvx truss"
            '';
          };
        });
    };
}
