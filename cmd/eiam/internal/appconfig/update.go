package appconfig

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/google/go-github/v33/github"
	"github.com/manifoldco/promptui"

	util "github.com/jessesomerville/ephemeral-iam/cmd/eiam/internal/eiamutil"
)

var (
	repoOwner = "jessesomerville"
	repoName  = "ephemeral-iam"

	// Version is the currently installed eiam client version.  This is populated by goreleaser when a new release is built
	Version = "v0.0.0"
)

func init() {
	if Version != "v0.0.0" {
		CheckForNewRelease()
	}
}

// CheckForNewRelease checks to see if there is a new version of eiam available
func CheckForNewRelease() {
	githubClient := github.NewClient(nil)
	releases, _, err := githubClient.Repositories.ListReleases(context.Background(), repoOwner, repoName, nil)
	if err != nil {
		util.Logger.Error("Unable to check for new eiam releases")
	}

	newestVersion := Version
	var newestRelease *github.RepositoryRelease
	for _, release := range releases {
		if semver.Compare(release.GetTagName(), newestVersion) > 0 {
			newestVersion = release.GetTagName()
			newestRelease = release
		}
	}

	if newestVersion == Version {
		util.Logger.Debugf("Newest version of eiam (%s) is currently installed", newestVersion)
	} else {
		util.Logger.Infof("Found a new version of eiam: %s (installed version is %s)", newestVersion, Version)
		updatePrompt := fmt.Sprintf("Would you like to install eiam %s now", newestVersion)
		prompt := promptui.Prompt{
			Label:     updatePrompt,
			IsConfirm: true,
		}

		if _, err := prompt.Run(); err == nil {
			util.Logger.Infof("Installing eiam %s", newestVersion)
			installNewVersion(newestRelease)
		}
	}
}

func installNewVersion(release *github.RepositoryRelease) {
	releaseType := formatArch()

	var downloadURL string
	for _, asset := range release.Assets {
		if strings.Contains(asset.GetName(), releaseType) {
			downloadURL = asset.GetBrowserDownloadURL()
			break
		}
	}
	if downloadURL == "" {
		util.Logger.Error("Failed to find a release version that matches your OS and architecture")
		util.Logger.Error("Skipping update, please try again later\n")
		return
	}

	if err := downloadAndExtract(downloadURL); err != nil {
		util.Logger.Error(err)
		util.Logger.Error("Skipping update, please try again later\n")
		return
	}

	installPath, err := util.CheckCommandExists("eiam")
	if err != nil {
		util.Logger.Error(err)
		util.Logger.Error("Skipping update, please try again later\n")
		return
	}

	tmpLoc := filepath.Join(os.TempDir(), "eiam")
	if err := os.Rename(tmpLoc, installPath); err != nil {
		util.Logger.Errorf("Failed to move %s to %s", tmpLoc, installPath)
		return
	}
	util.Logger.Info("Update completed successfully")
	os.Exit(0)
}

func downloadAndExtract(url string) error {
	util.Logger.Infof("Downloading new version from %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("Failed to download release from %s: %v", url, err)
	}
	defer resp.Body.Close()

	util.Logger.Info("Successfully downloaded the archive, now extracting its contents")
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {
		// if no more files are found return
		case err == io.EOF:
			return nil
		// return any other error
		case err != nil:
			return err
		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		target := filepath.Join(os.TempDir(), header.Name)

		// check the file type
		switch header.Typeflag {
		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0o755); err != nil {
					return err
				}
			}
		// if it's a file create it
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}
			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			f.Close()
		}
	}
}

func formatArch() string {
	var formatted string
	if runtime.GOOS == "linux" {
		formatted = "Linux"
	} else if runtime.GOOS == "darwin" {
		formatted = "Darwin_macOS"
	} else {
		util.Logger.Errorf("Failed to recognize system OS %s", runtime.GOOS)
		util.Logger.Fatal("Supported values are darwin and linux")
	}

	if runtime.GOARCH == "386" {
		return fmt.Sprintf("%s_i386", formatted)
	} else if runtime.GOARCH == "amd64" {
		return fmt.Sprintf("%s_x86_64", formatted)
	} else {
		return fmt.Sprintf("%s_arm64", formatted)
	}
}
