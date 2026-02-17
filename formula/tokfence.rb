class Tokfence < Formula
  desc "Local-first AI API gateway, key vault, and observability daemon"
  homepage "https://github.com/FoxOnTheRun42/tokfence"
  url "https://github.com/FoxOnTheRun42/tokfence/archive/refs/tags/v0.1.0-alpha.tar.gz"
  sha256 "e20359781e0a374fa395383106883be9805fdfa590b082239ebde00ccc4eb3f2"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/tokfence"
  end

  test do
    output = shell_output("#{bin}/tokfence --help")
    assert_match "tokfence", output
  end
end
