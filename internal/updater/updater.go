package updater

import (
	"context"
	"fmt"
	"runtime"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/guiyumin/vget/internal/core/version"
)

const (
	repoOwner = "guiyumin"
	repoName  = "vget"
)

// CheckUpdate checks if a new version is available
func CheckUpdate() (*selfupdate.Release, bool, error) {
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return nil, false, err
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source: source,
	})
	if err != nil {
		return nil, false, err
	}

	latest, found, err := updater.DetectLatest(context.Background(), selfupdate.NewRepositorySlug(repoOwner, repoName))
	if err != nil {
		return nil, false, fmt.Errorf("failed to check for updates: %w", err)
	}

	if !found {
		return nil, false, nil
	}

	currentVersion := version.Version
	// Remove 'v' prefix if present for comparison
	if len(currentVersion) > 0 && currentVersion[0] == 'v' {
		currentVersion = currentVersion[1:]
	}

	if latest.LessOrEqual(currentVersion) {
		return latest, false, nil
	}

	return latest, true, nil
}

// Update performs the self-update
func Update() error {
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return err
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source: source,
	})
	if err != nil {
		return err
	}

	latest, found, err := updater.DetectLatest(context.Background(), selfupdate.NewRepositorySlug(repoOwner, repoName))
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if !found {
		return fmt.Errorf("no releases found for %s/%s", repoOwner, repoName)
	}

	currentVersion := version.Version
	if len(currentVersion) > 0 && currentVersion[0] == 'v' {
		currentVersion = currentVersion[1:]
	}

	if latest.LessOrEqual(currentVersion) {
		fmt.Printf("Already up to date (v%s)\n", currentVersion)
		return nil
	}

	fmt.Printf("Updating from v%s to %s...\n", currentVersion, latest.Version())

	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	err = updater.UpdateTo(context.Background(), latest, exe)
	if err != nil {
		return fmt.Errorf("failed to update: %w", err)
	}

	fmt.Printf("Successfully updated to %s\n", latest.Version())
	return nil
}

// GetPlatformAssetName returns the expected asset name for the current platform
func GetPlatformAssetName() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	return fmt.Sprintf("vget_%s_%s", os, arch)
}
