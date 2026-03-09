class Gecko < Formula
  desc "FOSS-first Infrastructure as Code framework with the Scute language"
  homepage "https://gecko-iac.dev"
  url "https://github.com/gecko-iac/gecko/archive/refs/tags/v#{version}.tar.gz"
  sha256 :no_check # filled in by goreleaser on release
  license "Apache-2.0"
  head "https://github.com/gecko-iac/gecko.git", branch: "main"

  bottle do
    sha256 cellar: :any_skip_relocation, arm64_sequoia: :no_check
    sha256 cellar: :any_skip_relocation, arm64_sonoma:  :no_check
    sha256 cellar: :any_skip_relocation, sequoia:       :no_check
    sha256 cellar: :any_skip_relocation, sonoma:        :no_check
    sha256 cellar: :any_skip_relocation, x86_64_linux:  :no_check
  end

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X github.com/gecko-iac/gecko/cmd.Version=#{version}
      -X github.com/gecko-iac/gecko/cmd.Commit=#{tap.user}
      -X github.com/gecko-iac/gecko/cmd.Date=#{time.iso8601}
    ]
    system "go", "build", *std_go_args(ldflags:), "."
  end

  def caveats
    <<~EOS
      To get started, create a new Gecko project:
        gecko hatch my-project

      Documentation: https://gecko-iac.dev/docs
    EOS
  end

  test do
    assert_match "gecko v#{version}", shell_output("#{bin}/gecko --version")
    assert_match "Usage", shell_output("#{bin}/gecko help")
  end
end
