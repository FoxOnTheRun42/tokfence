class Tokfence < Formula
  desc "Local-first AI API gateway, key vault, and observability daemon"
  homepage "https://github.com/macfox/tokfence"
  url "https://github.com/macfox/tokfence/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "REPLACE_WITH_RELEASE_TARBALL_SHA256"
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
