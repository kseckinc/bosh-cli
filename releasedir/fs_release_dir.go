package releasedir

import (
	"os"
	gopath "path"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	semver "github.com/cppforlife/go-semi-semantic/version"

	boshrel "github.com/cloudfoundry/bosh-init/release"
)

var (
	DefaultFinalVersion   = semver.MustNewVersionFromString("0")
	DefaultDevVersion     = semver.MustNewVersionFromString("0+dev.0")
	DefaultDevPostRelease = semver.MustNewVersionSegmentFromString("dev.1")
)

type FSReleaseDir struct {
	dirPath string

	config    Config
	gitRepo   GitRepo
	blobsDir  BlobsDir
	generator Generator

	devReleases   ReleaseIndex
	finalReleases ReleaseIndex
	finalIndicies boshrel.ArchiveIndicies

	releaseReader        boshrel.Reader
	releaseArchiveWriter boshrel.Writer

	fs boshsys.FileSystem
}

func NewFSReleaseDir(
	dirPath string,
	config Config,
	gitRepo GitRepo,
	blobsDir BlobsDir,
	generator Generator,
	devReleases ReleaseIndex,
	finalReleases ReleaseIndex,
	finalIndicies boshrel.ArchiveIndicies,
	releaseReader boshrel.Reader,
	releaseArchiveWriter boshrel.Writer,
	fs boshsys.FileSystem,
) FSReleaseDir {
	return FSReleaseDir{
		dirPath: dirPath,

		config:    config,
		gitRepo:   gitRepo,
		blobsDir:  blobsDir,
		generator: generator,

		devReleases:   devReleases,
		finalReleases: finalReleases,
		finalIndicies: finalIndicies,

		releaseReader:        releaseReader,
		releaseArchiveWriter: releaseArchiveWriter,

		fs: fs,
	}
}

func (d FSReleaseDir) Init(git bool) error {
	for _, name := range []string{"jobs", "packages", "src"} {
		err := d.fs.MkdirAll(gopath.Join(d.dirPath, name), os.ModePerm)
		if err != nil {
			return bosherr.WrapErrorf(err, "Creating %s/", name)
		}
	}

	err := d.config.SaveFinalName(gopath.Base(d.dirPath))
	if err != nil {
		return err
	}

	err = d.blobsDir.Init()
	if err != nil {
		return bosherr.WrapErrorf(err, "Initing blobs")
	}

	if git {
		err = d.gitRepo.Init()
		if err != nil {
			return err
		}
	}

	return nil
}

func (d FSReleaseDir) GenerateJob(name string) error {
	return d.generator.GenerateJob(name)
}

func (d FSReleaseDir) GeneratePackage(name string) error {
	return d.generator.GeneratePackage(name)
}

func (d FSReleaseDir) Reset() error {
	for _, name := range []string{".dev_builds", "dev_releases"} {
		err := d.fs.RemoveAll(gopath.Join(d.dirPath, name))
		if err != nil {
			return bosherr.WrapErrorf(err, "Removing %s/", name)
		}
	}

	return nil
}

func (d FSReleaseDir) DefaultName() (string, error) {
	return d.config.FinalName()
}

func (d FSReleaseDir) NextFinalVersion(name string) (semver.Version, error) {
	lastVer, err := d.finalReleases.LastVersion(name)
	if err != nil {
		return semver.Version{}, err
	} else if lastVer == nil {
		return DefaultFinalVersion, nil
	}

	incVer, err := lastVer.IncrementRelease()
	if err != nil {
		return semver.Version{}, bosherr.WrapErrorf(err, "Incrementing last final version")
	}

	return incVer, nil
}

func (d FSReleaseDir) NextDevVersion(name string, timestamp bool) (semver.Version, error) {
	// todo timestamp

	lastVer, _, err := d.lastDevOrFinalVersion(name)
	if err != nil {
		return semver.Version{}, err
	} else if lastVer == nil {
		lastVer = &DefaultDevVersion
	}

	incVer, err := lastVer.IncrementPostRelease(DefaultDevPostRelease)
	if err != nil {
		return semver.Version{}, bosherr.WrapErrorf(err, "Incrementing last dev version")
	}

	return incVer, nil
}

func (d FSReleaseDir) LastRelease() (boshrel.Release, error) {
	name, err := d.DefaultName()
	if err != nil {
		return nil, err
	}

	lastVer, relIndex, err := d.lastDevOrFinalVersion(name)
	if err != nil {
		return nil, err
	} else if lastVer == nil {
		return nil, bosherr.Errorf("Expected to find at least one dev or final version")
	}

	return d.releaseReader.Read(relIndex.ManifestPath(name, lastVer.AsString()))
}

func (d FSReleaseDir) BuildRelease(name string, version semver.Version, force bool) (boshrel.Release, error) {
	dirty, err := d.gitRepo.MustNotBeDirty(force)
	if err != nil {
		return nil, err
	}

	commitSHA, err := d.gitRepo.LastCommitSHA()
	if err != nil {
		return nil, err
	}

	err = d.blobsDir.DownloadBlobs()
	if err != nil {
		return nil, err
	}

	release, err := d.releaseReader.Read(d.dirPath)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Building a release from directory '%s'", d.dirPath)
	}

	release.SetName(name)
	release.SetVersion(version.AsString())
	release.SetCommitHash(commitSHA)
	release.SetUncommittedChanges(dirty)

	err = d.devReleases.Add(release.Manifest())
	if err != nil {
		return nil, err
	}

	return release, nil
}

func (d FSReleaseDir) FinalizeRelease(release boshrel.Release, force bool) error {
	_, err := d.gitRepo.MustNotBeDirty(force)
	if err != nil {
		return err
	}

	found, err := d.finalReleases.Contains(release)
	if err != nil {
		return err
	} else if found {
		return bosherr.Errorf("Release '%s' version '%s' already exists", release.Name(), release.Version())
	}

	err = release.Finalize(d.finalIndicies)
	if err != nil {
		return err
	}

	return d.finalReleases.Add(release.Manifest())
}

func (d FSReleaseDir) BuildReleaseArchive(release boshrel.Release) (string, error) {
	path, err := d.releaseArchiveWriter.Write(release, nil)
	if err != nil {
		return "", err
	}

	ver, err := semver.NewVersionFromString(release.Version())
	if err != nil {
		return "", err
	}

	var relIndex ReleaseIndex

	if strings.Contains(ver.PostRelease.AsString(), "dev") {
		relIndex = d.devReleases
	} else {
		relIndex = d.finalReleases
	}

	dstPath, err := relIndex.ArchivePath(release)
	if err != nil {
		return "", err
	}

	err = d.fs.Rename(path, dstPath)
	if err != nil {
		return "", bosherr.WrapErrorf(err, "Moving release archive to final destination")
	}

	return dstPath, nil
}

func (d FSReleaseDir) lastDevOrFinalVersion(name string) (*semver.Version, ReleaseIndex, error) {
	lastDevVer, err := d.devReleases.LastVersion(name)
	if err != nil {
		return nil, nil, err
	}

	lastFinalVer, err := d.finalReleases.LastVersion(name)
	if err != nil {
		return nil, nil, err
	}

	switch {
	case lastDevVer != nil && lastFinalVer != nil:
		if lastFinalVer.IsGt(*lastDevVer) {
			return lastFinalVer, d.finalReleases, nil
		} else {
			return lastDevVer, d.devReleases, nil
		}
	case lastDevVer != nil && lastFinalVer == nil:
		return lastDevVer, d.devReleases, nil
	case lastDevVer == nil && lastFinalVer != nil:
		return lastFinalVer, d.finalReleases, nil
	default:
		return nil, nil, nil
	}
}