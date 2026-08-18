# SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
# SPDX-License-Identifier: GPL-3.0-only
#
# Copy into a tap as Formula/tunny.rb. homebrew-core is not an option yet: it
# asks for 30 days of repository age and 75 stars, 225 when the author opens
# the pull request themselves.
class Tunny < Formula
  desc "Pick your admin SSH tunnel from a list instead of remembering its name"
  homepage "https://github.com/MorganKryze/tunny"
  url "https://github.com/MorganKryze/tunny/archive/refs/tags/v0.3.0.tar.gz"
  sha256 "b224ea6404d8f7fb98792207f71efc218e2a329465dd1850219aa42dad234c54"
  license "GPL-3.0-only"
  head "https://github.com/MorganKryze/tunny.git", branch: "main"

  depends_on "go" => :build

  def install
    # std_go_args supplies -trimpath and -o bin/tunny. The -X is not optional:
    # a tarball carries no VCS metadata, so without it the binary reports "dev".
    system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=v#{version}"), "./cmd/tunny"

    man1.install "docs/tunny.1"
    bash_completion.install "completions/tunny.bash" => "tunny"
    zsh_completion.install "completions/tunny.zsh" => "_tunny"
    fish_completion.install "completions/tunny.fish"
    pkgshare.install "destinations.example.toml"
  end

  def caveats
    <<~EOS
      tunny needs an OpenSSH client on your PATH; macOS ships one.

      Start from the example config:
        mkdir -p ~/.config/tunny
        cp #{opt_pkgshare}/destinations.example.toml ~/.config/tunny/destinations.toml
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tunny --version")

    # --preview reads a config and draws the picker without opening a tunnel
    # or writing any state, which makes it the one command safe to run here.
    (testpath/"cfg/tunny").mkpath
    (testpath/"cfg/tunny/destinations.toml").write <<~TOML
      [[destination]]
      name = "example"
      desc = "a test destination"
      host = "nowhere.invalid"
      forward = [{ local = 19999, to = "127.0.0.1:1", label = "Thing" }]
    TOML
    with_env(XDG_CONFIG_HOME: testpath/"cfg", XDG_STATE_HOME: testpath/"state") do
      assert_match "example", shell_output("#{bin}/tunny --preview --width 80")
      assert_equal "example\n", shell_output("#{bin}/tunny --list")
    end
  end
end
