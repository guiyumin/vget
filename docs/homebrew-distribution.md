# Homebrew Distribution

This document explains how to distribute vget via Homebrew.

## Option 1: Own Tap (Recommended)

Use your own Homebrew tap for instant updates with no PR reviews.

### Setup

1. Create GitHub repo: `guiyumin/homebrew-tap`

2. Add `Formula/vget.rb`:

```ruby
class Vget < Formula
  desc "Media downloader CLI for various platforms"
  homepage "https://github.com/guiyumin/vget"
  url "https://github.com/guiyumin/vget/archive/refs/tags/v0.9.2.tar.gz"
  sha256 "bf5228673cfd080ac8f0e9d0ee05e875fc5bfcde342ae5fd615c5d2a23181ab3"
  license "Apache-2.0"

  depends_on "go" => :build

  def install
    ldflags = "-s -w -X github.com/guiyumin/vget/internal/version.Version=#{version}"
    system "go", "build", *std_go_args(ldflags: ldflags), "./cmd/vget"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/vget --version")
  end
end
```

3. Users install with:

```bash
brew tap guiyumin/tap
brew install vget
```

Or in one command:

```bash
brew install guiyumin/tap/vget
```

### Automate Updates with GoReleaser

Add to `.goreleaser.yaml`:

```yaml
brews:
  - repository:
      owner: guiyumin
      name: homebrew-tap
    homepage: "https://github.com/guiyumin/vget"
    description: "Media downloader CLI for various platforms"
    license: "Apache-2.0"
    directory: Formula
```

This automatically updates your tap on every GitHub release.

## Option 2: homebrew-core (Official)

Submit to the official Homebrew repository for `brew install vget` (no tap needed).

### Requirements

- 30+ GitHub stars (vget has 300+ ✓)
- Stable versioned releases ✓
- Open source license ✓

### Submission Steps

```bash
# 1. Fork homebrew-core on GitHub, then clone
git clone https://github.com/YOUR_USERNAME/homebrew-core.git
cd homebrew-core

# 2. Create the formula
mkdir -p Formula/v
# Add Formula/v/vget.rb (same content as above)

# 3. Test locally
brew install --build-from-source ./Formula/v/vget.rb
brew test vget
brew audit --strict --new vget

# 4. Commit and push
git checkout -b vget
git add Formula/v/vget.rb
git commit -m "vget: new formula"
git push origin vget

# 5. Open PR to Homebrew/homebrew-core
```

PR title: `vget 0.9.2 (new formula)`

### Updating Versions

After initial approval, update with:

```bash
brew bump-formula-pr --url https://github.com/guiyumin/vget/archive/refs/tags/vX.Y.Z.tar.gz vget
```

Or automate with GitHub Actions (`.github/workflows/homebrew.yml`):

```yaml
name: Bump Homebrew Formula

on:
  release:
    types: [published]

jobs:
  bump-formula:
    runs-on: macos-latest
    steps:
      - name: Bump formula
        uses: mislav/bump-homebrew-formula-action@v3
        with:
          formula-name: vget
        env:
          COMMITTER_TOKEN: ${{ secrets.HOMEBREW_GITHUB_TOKEN }}
```

Setup: Create a GitHub PAT with `public_repo` scope and add as `HOMEBREW_GITHUB_TOKEN` secret.

Version bump PRs are auto-merged by BrewTestBot in ~5-15 minutes (no human review).

## Comparison

| Approach | Review Time | User Command |
|----------|-------------|--------------|
| Own tap | Instant | `brew install guiyumin/tap/vget` |
| homebrew-core | ~5-15 min (auto-merge) | `brew install vget` |
