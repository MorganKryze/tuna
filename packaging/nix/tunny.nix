# SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
# SPDX-License-Identifier: GPL-3.0-only
#
# For pkgs/by-name/tu/tunny/package.nix.
{
  lib,
  buildGoModule,
  fetchFromGitHub,
  installShellFiles,
  makeWrapper,
  openssh,
}:

buildGoModule (finalAttrs: {
  pname = "tunny";
  version = "0.2.0";

  src = fetchFromGitHub {
    owner = "MorganKryze";
    repo = "tunny";
    tag = "v${finalAttrs.version}";
    hash = "sha256-PY721M+2B0R1cHaBvSx+TELynR1qj1qbWoNNf0T8IIA=";
  };

  vendorHash = "sha256-wXOe0s7HqXmpEUx+bWe17epdPf6GJ1G3i8RyU87gXXY=";

  subPackages = [ "cmd/tunny" ];

  # A source tarball carries no VCS metadata, so without this the binary
  # reports "dev".
  ldflags = [
    "-s"
    "-w"
    "-X main.version=v${finalAttrs.version}"
  ];

  nativeBuildInputs = [
    installShellFiles
    makeWrapper
  ];

  # tunny executes ssh from PATH and implements no SSH of its own.
  postInstall = ''
    wrapProgram $out/bin/tunny --prefix PATH : ${lib.makeBinPath [ openssh ]}
    installManPage docs/tunny.1
    installShellCompletion \
      --bash completions/tunny.bash \
      --zsh completions/tunny.zsh \
      --fish completions/tunny.fish
    install -Dm644 destinations.example.toml \
      $out/share/doc/tunny/examples/destinations.example.toml
  '';

  # No network, no TTY, no ssh server.
  doCheck = true;

  meta = {
    description = "Pick your admin SSH tunnel from a list instead of remembering its name";
    homepage = "https://github.com/MorganKryze/tunny";
    changelog = "https://github.com/MorganKryze/tunny/blob/v${finalAttrs.version}/CHANGELOG.md";
    license = lib.licenses.gpl3Only;
    maintainers = [ ];
    mainProgram = "tunny";
    # Not lib.platforms.unix: solaris, illumos and aix have no termios path
    # here and fail to compile.
    platforms = lib.platforms.linux ++ lib.platforms.darwin ++ lib.platforms.freebsd ++ lib.platforms.netbsd ++ lib.platforms.openbsd;
  };
})
