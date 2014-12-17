package integration_test

import (
	. "github.com/cloudfoundry/bosh-micro-cli/cmd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"errors"
	"io/ioutil"
	"net/http"
	"os"

	"code.google.com/p/gomock/gomock"
	mock_cloud "github.com/cloudfoundry/bosh-micro-cli/cloud/mocks"
	mock_cpi "github.com/cloudfoundry/bosh-micro-cli/cpi/mocks"
	mock_agentclient "github.com/cloudfoundry/bosh-micro-cli/deployment/agentclient/mocks"

	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	fakesys "github.com/cloudfoundry/bosh-agent/system/fakes"
	fakeuuid "github.com/cloudfoundry/bosh-agent/uuid/fakes"

	bmconfig "github.com/cloudfoundry/bosh-micro-cli/config"
	bmcpi "github.com/cloudfoundry/bosh-micro-cli/cpi"
	bmdepl "github.com/cloudfoundry/bosh-micro-cli/deployment"
	bmac "github.com/cloudfoundry/bosh-micro-cli/deployment/agentclient"
	bmas "github.com/cloudfoundry/bosh-micro-cli/deployment/applyspec"
	bmdisk "github.com/cloudfoundry/bosh-micro-cli/deployment/disk"
	bmhttp "github.com/cloudfoundry/bosh-micro-cli/deployment/httpclient"
	bminstance "github.com/cloudfoundry/bosh-micro-cli/deployment/instance"
	bmmanifest "github.com/cloudfoundry/bosh-micro-cli/deployment/manifest"
	bmdeplval "github.com/cloudfoundry/bosh-micro-cli/deployment/manifest/validator"
	bmsshtunnel "github.com/cloudfoundry/bosh-micro-cli/deployment/sshtunnel"
	bmstemcell "github.com/cloudfoundry/bosh-micro-cli/deployment/stemcell"
	bmvm "github.com/cloudfoundry/bosh-micro-cli/deployment/vm"
	bmeventlog "github.com/cloudfoundry/bosh-micro-cli/eventlogger"
	bmregistry "github.com/cloudfoundry/bosh-micro-cli/registry"
	bmrel "github.com/cloudfoundry/bosh-micro-cli/release"

	fakebmcpi "github.com/cloudfoundry/bosh-micro-cli/cpi/fakes"
	fakebmcrypto "github.com/cloudfoundry/bosh-micro-cli/crypto/fakes"
	fakebmas "github.com/cloudfoundry/bosh-micro-cli/deployment/applyspec/fakes"
	fakebmstemcell "github.com/cloudfoundry/bosh-micro-cli/deployment/stemcell/fakes"
	fakeui "github.com/cloudfoundry/bosh-micro-cli/ui/fakes"
)

var _ = Describe("bosh-micro", func() {
	var mockCtrl *gomock.Controller

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("Deploy", func() {
		var (
			fs     *fakesys.FakeFileSystem
			logger boshlog.Logger

			mockCPIDeploymentFactory *mock_cpi.MockDeploymentFactory
			registryServerManager    bmregistry.ServerManager

			fakeCPIInstaller        *fakebmcpi.FakeInstaller
			fakeStemcellExtractor   *fakebmstemcell.FakeExtractor
			fakeUUIDGenerator       *fakeuuid.FakeGenerator
			fakeSHA1Calculator      *fakebmcrypto.FakeSha1Calculator
			deploymentConfigService bmconfig.DeploymentConfigService
			vmRepo                  bmconfig.VMRepo
			diskRepo                bmconfig.DiskRepo
			stemcellRepo            bmconfig.StemcellRepo
			deploymentRepo          bmconfig.DeploymentRepo
			releaseRepo             bmconfig.ReleaseRepo
			userConfig              bmconfig.UserConfig

			sshTunnelFactory bmsshtunnel.Factory

			diskManagerFactory bmdisk.ManagerFactory
			diskDeployer       bminstance.DiskDeployer

			ui          *fakeui.FakeUI
			eventLogger bmeventlog.EventLogger

			stemcellManagerFactory bmstemcell.ManagerFactory
			vmManagerFactory       bmvm.ManagerFactory

			fakeApplySpecFactory       *fakebmas.FakeApplySpecFactory
			fakeTemplatesSpecGenerator *fakebmas.FakeTemplatesSpecGenerator
			applySpec                  bmas.ApplySpec

			mockAgentClient        *mock_agentclient.MockAgentClient
			mockAgentClientFactory *mock_agentclient.MockFactory
			mockCloud              *mock_cloud.MockCloud
			deploymentManifestPath = "/deployment-dir/fake-deployment-manifest.yml"
			deploymentConfigPath   = "/fake-bosh-deployments.json"

			cloudProperties   = map[string]interface{}{}
			stemcellImagePath = "fake-stemcell-image-path"
			stemcellCID       = "fake-stemcell-cid"
			env               = map[string]interface{}{}
			networksSpec      = map[string]interface{}{
				"network-1": map[string]interface{}{
					"type":             "dynamic",
					"ip":               "",
					"cloud_properties": cloudProperties,
				},
			}
			agentRunningState = bmac.State{JobState: "running"}
			mbusURL           = "http://fake-mbus-url"
		)

		var writeDeploymentManifest = func() {
			err := fs.WriteFileString(deploymentManifestPath, `---
name: test-release

networks:
- name: network-1
  type: dynamic

resource_pools:
- name: resource-pool-1
  network: network-1

jobs:
- name: cpi
  instances: 1
  persistent_disk: 1024
  networks:
  - name: network-1

cloud_provider:
  mbus: http://fake-mbus-url
  registry:
    host: 127.0.0.1
    port: 6301
    username: fake-registry-user
    password: fake-registry-password
`)
			Expect(err).ToNot(HaveOccurred())

			fakeSHA1Calculator.SetCalculateBehavior(map[string]fakebmcrypto.CalculateInput{
				deploymentManifestPath: {Sha1: "fake-deployment-sha1-1"},
			})
		}

		var writeDeploymentManifestWithLargerDisk = func() {
			err := fs.WriteFileString(deploymentManifestPath, `---
name: test-release

networks:
- name: network-1
  type: dynamic

resource_pools:
- name: resource-pool-1
  network: network-1

jobs:
- name: cpi
  instances: 1
  persistent_disk: 2048
  networks:
  - name: network-1

cloud_provider:
  mbus: http://fake-mbus-url
  registry:
    host: 127.0.0.1
    port: 6301
    username: fake-registry-user
    password: fake-registry-password
`)
			Expect(err).ToNot(HaveOccurred())

			fakeSHA1Calculator.SetCalculateBehavior(map[string]fakebmcrypto.CalculateInput{
				deploymentManifestPath: {Sha1: "fake-deployment-sha1-2"},
			})
		}

		var writeCPIReleaseTarball = func() {
			err := fs.WriteFileString("/fake-cpi-release.tgz", "fake-tgz-content")
			Expect(err).ToNot(HaveOccurred())
		}

		var allowCPIToBeInstalled = func() {
			cpiRelease := bmrel.NewRelease(
				"fake-cpi-release-name",
				"fake-cpi-release-version",
				[]bmrel.Job{},
				[]*bmrel.Package{},
				"fake-cpi-extracted-dir",
				fs,
			)
			fakeCPIInstaller.SetExtractBehavior("/fake-cpi-release.tgz", func(releaseTarballPath string) (bmrel.Release, error) {
				err := fs.MkdirAll("fake-cpi-extracted-dir", os.ModePerm)
				return cpiRelease, err
			})

			cpiDeploymentManifest := bmmanifest.CPIDeploymentManifest{
				Name: "test-release",
				Mbus: mbusURL,
				Registry: bmmanifest.Registry{
					Username: "fake-registry-user",
					Password: "fake-registry-password",
					Host:     "127.0.0.1",
					Port:     6301,
				},
			}
			fakeCPIInstaller.SetInstallBehavior(cpiDeploymentManifest, cpiRelease, mockCloud, nil)

			cpiDeployment := bmcpi.NewDeployment(cpiDeploymentManifest, registryServerManager, fakeCPIInstaller)
			mockCPIDeploymentFactory.EXPECT().NewDeployment(cpiDeploymentManifest).Return(cpiDeployment).AnyTimes()
		}

		var writeStemcellReleaseTarball = func() {
			err := fs.WriteFileString("/fake-stemcell-release.tgz", "fake-tgz-content")
			Expect(err).ToNot(HaveOccurred())
		}

		var allowStemcellToBeExtracted = func() {
			stemcellManifest := bmstemcell.Manifest{
				ImagePath: "fake-stemcell-image-path",
				Name:      "fake-stemcell-name",
				Version:   "fake-stemcell-version",
				SHA1:      "fake-stemcell-sha1",
			}
			stemcellApplySpec := bmstemcell.ApplySpec{
				Job: bmstemcell.Job{
					Name:      "cpi",
					Templates: []bmstemcell.Blob{},
				},
				Packages: map[string]bmstemcell.Blob{},
				Networks: map[string]interface{}{},
			}
			extractedStemcell := bmstemcell.NewExtractedStemcell(
				stemcellManifest,
				stemcellApplySpec,
				"fake-stemcell-extracted-dir",
				fs,
			)
			fakeStemcellExtractor.SetExtractBehavior("/fake-stemcell-release.tgz", extractedStemcell, nil)
		}

		var allowApplySpecToBeCreated = func() {
			applySpec = bmas.ApplySpec{
				Deployment: "",
				Index:      0,
				Packages:   map[string]bmas.Blob{},
				Networks:   map[string]interface{}{},
				Job:        bmas.Job{},
				RenderedTemplatesArchive: bmas.RenderedTemplatesArchiveSpec{},
				ConfigurationHash:        "",
			}
			fakeApplySpecFactory.CreateApplySpec = applySpec
		}

		var newDeployCmd = func() Cmd {
			deploymentParser := bmmanifest.NewParser(fs, logger)

			boshDeploymentValidator := bmdeplval.NewBoshDeploymentValidator()

			deploymentRecord := bmdepl.NewDeploymentRecord(deploymentRepo, releaseRepo, stemcellRepo, fakeSHA1Calculator)

			deployer := bmdepl.NewDeployer(
				stemcellManagerFactory,
				vmManagerFactory,
				sshTunnelFactory,
				diskDeployer,
				eventLogger,
				logger,
			)

			deploymentFactory := bmdepl.NewFactory(deployer)

			return NewDeployCmd(
				ui,
				userConfig,
				fs,
				deploymentParser,
				boshDeploymentValidator,
				mockCPIDeploymentFactory,
				fakeStemcellExtractor,
				deploymentRecord,
				deploymentFactory,
				eventLogger,
				logger,
			)
		}

		var expectDeployFlow = func() {
			vmCID := "fake-vm-cid-1"
			diskCID := "fake-disk-cid-1"
			diskSize := 1024

			gomock.InOrder(
				mockCloud.EXPECT().CreateStemcell(cloudProperties, stemcellImagePath).Return(stemcellCID, nil),
				mockCloud.EXPECT().CreateVM(stemcellCID, cloudProperties, networksSpec, env).Return(vmCID, nil),
				mockAgentClient.EXPECT().Ping().Return("any-state", nil),

				mockCloud.EXPECT().CreateDisk(diskSize, cloudProperties, vmCID).Return(diskCID, nil),
				mockCloud.EXPECT().AttachDisk(vmCID, diskCID),
				mockAgentClient.EXPECT().MountDisk(diskCID),

				mockAgentClient.EXPECT().Stop(),
				mockAgentClient.EXPECT().Apply(applySpec),
				mockAgentClient.EXPECT().Start(),
				mockAgentClient.EXPECT().GetState().Return(agentRunningState, nil),
			)
		}

		var expectDeployWithDiskMigration = func() {
			oldVMCID := "fake-vm-cid-1"
			newVMCID := "fake-vm-cid-2"
			oldDiskCID := "fake-disk-cid-1"
			newDiskCID := "fake-disk-cid-2"
			newDiskSize := 2048

			gomock.InOrder(
				// shutdown old vm
				mockAgentClient.EXPECT().Ping().Return("any-state", nil),
				mockAgentClient.EXPECT().Stop(),
				mockAgentClient.EXPECT().ListDisk().Return([]string{oldDiskCID}, nil),
				mockAgentClient.EXPECT().UnmountDisk(oldDiskCID),
				mockCloud.EXPECT().DeleteVM(oldVMCID),

				// create new vm
				mockCloud.EXPECT().CreateVM(stemcellCID, cloudProperties, networksSpec, env).Return(newVMCID, nil),
				mockAgentClient.EXPECT().Ping().Return("any-state", nil),

				// attach both disks and migrate
				mockCloud.EXPECT().AttachDisk(newVMCID, oldDiskCID),
				mockAgentClient.EXPECT().MountDisk(oldDiskCID),
				mockCloud.EXPECT().CreateDisk(newDiskSize, cloudProperties, newVMCID).Return(newDiskCID, nil),
				mockCloud.EXPECT().AttachDisk(newVMCID, newDiskCID),
				mockAgentClient.EXPECT().MountDisk(newDiskCID),
				mockAgentClient.EXPECT().MigrateDisk(),
				mockCloud.EXPECT().DetachDisk(newVMCID, oldDiskCID),
				mockCloud.EXPECT().DeleteDisk(oldDiskCID),

				// start jobs & wait for running
				mockAgentClient.EXPECT().Stop(),
				mockAgentClient.EXPECT().Apply(applySpec),
				mockAgentClient.EXPECT().Start(),
				mockAgentClient.EXPECT().GetState().Return(agentRunningState, nil),
			)
		}

		var expectDeployWithDiskMigrationNoVMShutdown = func() {
			oldVMCID := "fake-vm-cid-1"
			newVMCID := "fake-vm-cid-2"
			oldDiskCID := "fake-disk-cid-1"
			newDiskCID := "fake-disk-cid-2"
			newDiskSize := 2048

			gomock.InOrder(
				// shutdown old vm (without talking to agent)
				mockCloud.EXPECT().DeleteVM(oldVMCID),

				// create new vm
				mockCloud.EXPECT().CreateVM(stemcellCID, cloudProperties, networksSpec, env).Return(newVMCID, nil),
				mockAgentClient.EXPECT().Ping().Return("any-state", nil),

				// attach both disks and migrate
				mockCloud.EXPECT().AttachDisk(newVMCID, oldDiskCID),
				mockAgentClient.EXPECT().MountDisk(oldDiskCID),
				mockCloud.EXPECT().CreateDisk(newDiskSize, cloudProperties, newVMCID).Return(newDiskCID, nil),
				mockCloud.EXPECT().AttachDisk(newVMCID, newDiskCID),
				mockAgentClient.EXPECT().MountDisk(newDiskCID),
				mockAgentClient.EXPECT().MigrateDisk(),
				mockCloud.EXPECT().DetachDisk(newVMCID, oldDiskCID),
				mockCloud.EXPECT().DeleteDisk(oldDiskCID),

				// start jobs & wait for running
				mockAgentClient.EXPECT().Stop(),
				mockAgentClient.EXPECT().Apply(applySpec),
				mockAgentClient.EXPECT().Start(),
				mockAgentClient.EXPECT().GetState().Return(agentRunningState, nil),
			)
		}

		var expectDeployWithDiskMigrationFailure = func() {
			oldVMCID := "fake-vm-cid-1"
			newVMCID := "fake-vm-cid-2"
			oldDiskCID := "fake-disk-cid-1"
			newDiskCID := "fake-disk-cid-2"
			newDiskSize := 2048

			gomock.InOrder(
				// shutdown old vm
				mockAgentClient.EXPECT().Ping().Return("any-state", nil),
				mockAgentClient.EXPECT().Stop(),
				mockAgentClient.EXPECT().ListDisk().Return([]string{oldDiskCID}, nil),
				mockAgentClient.EXPECT().UnmountDisk(oldDiskCID),
				mockCloud.EXPECT().DeleteVM(oldVMCID),

				// create new vm
				mockCloud.EXPECT().CreateVM(stemcellCID, cloudProperties, networksSpec, env).Return(newVMCID, nil),
				mockAgentClient.EXPECT().Ping().Return("any-state", nil),

				// attach both disks and migrate (with error
				mockCloud.EXPECT().AttachDisk(newVMCID, oldDiskCID),
				mockAgentClient.EXPECT().MountDisk(oldDiskCID),
				mockCloud.EXPECT().CreateDisk(newDiskSize, cloudProperties, newVMCID).Return(newDiskCID, nil),
				mockCloud.EXPECT().AttachDisk(newVMCID, newDiskCID),
				mockAgentClient.EXPECT().MountDisk(newDiskCID),
				mockAgentClient.EXPECT().MigrateDisk().Return(errors.New("fake-migration-error")),
			)
		}

		var expectDeployWithDiskMigrationRepair = func() {
			oldVMCID := "fake-vm-cid-2"
			newVMCID := "fake-vm-cid-3"
			oldDiskCID := "fake-disk-cid-1"
			newDiskCID := "fake-disk-cid-3"
			newDiskSize := 2048

			gomock.InOrder(
				// shutdown old vm
				mockAgentClient.EXPECT().Ping().Return("any-state", nil),
				mockAgentClient.EXPECT().Stop(),
				mockAgentClient.EXPECT().ListDisk().Return([]string{oldDiskCID}, nil),
				mockAgentClient.EXPECT().UnmountDisk(oldDiskCID),
				mockCloud.EXPECT().DeleteVM(oldVMCID),

				// create new vm
				mockCloud.EXPECT().CreateVM(stemcellCID, cloudProperties, networksSpec, env).Return(newVMCID, nil),
				mockAgentClient.EXPECT().Ping().Return("any-state", nil),

				// attach both disks and migrate
				mockCloud.EXPECT().AttachDisk(newVMCID, oldDiskCID),
				mockAgentClient.EXPECT().MountDisk(oldDiskCID),
				mockCloud.EXPECT().CreateDisk(newDiskSize, cloudProperties, newVMCID).Return(newDiskCID, nil),
				mockCloud.EXPECT().AttachDisk(newVMCID, newDiskCID),
				mockAgentClient.EXPECT().MountDisk(newDiskCID),
				mockAgentClient.EXPECT().MigrateDisk(),
				mockCloud.EXPECT().DetachDisk(newVMCID, oldDiskCID),
				mockCloud.EXPECT().DeleteDisk(oldDiskCID),

				// start jobs & wait for running
				mockAgentClient.EXPECT().Stop(),
				mockAgentClient.EXPECT().Apply(applySpec),
				mockAgentClient.EXPECT().Start(),
				mockAgentClient.EXPECT().GetState().Return(agentRunningState, nil),
			)
		}

		var expectRegistryToWork = func() {
			httpClient := bmhttp.NewHTTPClient(logger)

			endpoint := "http://fake-registry-user:fake-registry-password@127.0.0.1:6301/instances/fake-agent-id/settings"

			settingsBytes := []byte("fake-registry-contents") //usually json, but not required to be
			response, err := httpClient.Put(endpoint, settingsBytes)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusCreated))

			response, err = httpClient.Get(endpoint)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			responseBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(responseBytes).To(Equal([]byte("{\"settings\":\"fake-registry-contents\",\"status\":\"ok\"}")))

			response, err = httpClient.Delete(endpoint)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		}

		var expectDeployFlowWithRegistry = func() {
			vmCID := "fake-vm-cid-1"
			diskCID := "fake-disk-cid-1"
			diskSize := 1024

			gomock.InOrder(
				mockCloud.EXPECT().CreateStemcell(cloudProperties, stemcellImagePath).Do(
					func(_, _ interface{}) { expectRegistryToWork() },
				).Return(stemcellCID, nil),
				mockCloud.EXPECT().CreateVM(stemcellCID, cloudProperties, networksSpec, env).Do(
					func(_, _, _, _ interface{}) { expectRegistryToWork() },
				).Return(vmCID, nil),

				mockAgentClient.EXPECT().Ping().Return("any-state", nil),

				mockCloud.EXPECT().CreateDisk(diskSize, cloudProperties, vmCID).Do(
					func(_, _, _ interface{}) { expectRegistryToWork() },
				).Return(diskCID, nil),
				mockCloud.EXPECT().AttachDisk(vmCID, diskCID).Do(
					func(_, _ interface{}) { expectRegistryToWork() },
				),

				mockAgentClient.EXPECT().MountDisk(diskCID),
				mockAgentClient.EXPECT().Stop().Do(
					func() { expectRegistryToWork() },
				),
				mockAgentClient.EXPECT().Apply(applySpec),
				mockAgentClient.EXPECT().Start(),
				mockAgentClient.EXPECT().GetState().Return(agentRunningState, nil),
			)
		}

		BeforeEach(func() {
			fs = fakesys.NewFakeFileSystem()
			logger = boshlog.NewLogger(boshlog.LevelNone)
			deploymentConfigService = bmconfig.NewFileSystemDeploymentConfigService(deploymentConfigPath, fs, logger)
			fakeUUIDGenerator = fakeuuid.NewFakeGenerator()

			fakeSHA1Calculator = fakebmcrypto.NewFakeSha1Calculator()

			mockCPIDeploymentFactory = mock_cpi.NewMockDeploymentFactory(mockCtrl)

			sshTunnelFactory = bmsshtunnel.NewFactory(logger)

			config, err := deploymentConfigService.Load()
			Expect(err).ToNot(HaveOccurred())
			config.UUID = "fake-agent-id"
			err = deploymentConfigService.Save(config)
			Expect(err).ToNot(HaveOccurred())

			vmRepo = bmconfig.NewVMRepo(deploymentConfigService)
			diskRepo = bmconfig.NewDiskRepo(deploymentConfigService, fakeUUIDGenerator)
			stemcellRepo = bmconfig.NewStemcellRepo(deploymentConfigService, fakeUUIDGenerator)
			deploymentRepo = bmconfig.NewDeploymentRepo(deploymentConfigService)
			releaseRepo = bmconfig.NewReleaseRepo(deploymentConfigService, fakeUUIDGenerator)

			diskManagerFactory = bmdisk.NewManagerFactory(diskRepo, logger)
			diskDeployer = bminstance.NewDiskDeployer(diskManagerFactory, diskRepo, logger)

			mockCloud = mock_cloud.NewMockCloud(mockCtrl)

			registryServerManager = bmregistry.NewServerManager(logger)

			fakeCPIInstaller = fakebmcpi.NewFakeInstaller()
			fakeStemcellExtractor = fakebmstemcell.NewFakeExtractor()

			ui = &fakeui.FakeUI{}
			eventLogger = bmeventlog.NewEventLogger(ui)

			mockAgentClientFactory = mock_agentclient.NewMockFactory(mockCtrl)
			mockAgentClient = mock_agentclient.NewMockAgentClient(mockCtrl)

			stemcellManagerFactory = bmstemcell.NewManagerFactory(stemcellRepo, eventLogger)

			fakeApplySpecFactory = fakebmas.NewFakeApplySpecFactory()
			fakeTemplatesSpecGenerator = fakebmas.NewFakeTemplatesSpecGenerator()

			vmManagerFactory = bmvm.NewManagerFactory(
				vmRepo,
				stemcellRepo,
				mockAgentClientFactory,
				fakeApplySpecFactory,
				fakeTemplatesSpecGenerator,
				fs,
				logger,
			)

			userConfig = bmconfig.UserConfig{DeploymentFile: deploymentManifestPath}

			mockAgentClientFactory.EXPECT().Create(mbusURL).Return(mockAgentClient).AnyTimes()

			writeDeploymentManifest()
			writeCPIReleaseTarball()
			allowCPIToBeInstalled()

			writeStemcellReleaseTarball()
			allowStemcellToBeExtracted()
			allowApplySpecToBeCreated()
		})

		Context("when the deployment has not been set", func() {
			BeforeEach(func() {
				userConfig.DeploymentFile = ""
			})

			It("returns an error", func() {
				err := newDeployCmd().Run([]string{"/fake-cpi-release.tgz", "/fake-stemcell-release.tgz"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("No deployment set"))
			})
		})

		Context("when the deployment config file does not exist", func() {
			BeforeEach(func() {
				err := fs.RemoveAll(deploymentConfigPath)
				Expect(err).ToNot(HaveOccurred())
			})

			It("creates one", func() {
				expectDeployFlow()

				err := newDeployCmd().Run([]string{"/fake-cpi-release.tgz", "/fake-stemcell-release.tgz"})
				Expect(err).ToNot(HaveOccurred())

				Expect(fs.FileExists(deploymentConfigPath)).To(BeTrue())
			})
		})

		Context("when the deployment has been deployed", func() {
			var (
				expectHasVM1 *gomock.Call
			)
			BeforeEach(func() {
				expectDeployFlow()

				err := newDeployCmd().Run([]string{"/fake-cpi-release.tgz", "/fake-stemcell-release.tgz"})
				Expect(err).ToNot(HaveOccurred())

				// reset output buffer
				ui.Said = []string{}

				// after cloud.CreateVM, cloud.HasVM should return true
				expectHasVM1 = mockCloud.EXPECT().HasVM("fake-vm-cid-1").Return(true, nil)
			})

			Context("when persistent disk size is increased", func() {
				BeforeEach(func() {
					writeDeploymentManifestWithLargerDisk()
				})

				It("migrates the disk content", func() {
					expectDeployWithDiskMigration()

					err := newDeployCmd().Run([]string{"/fake-cpi-release.tgz", "/fake-stemcell-release.tgz"})
					Expect(err).ToNot(HaveOccurred())
				})

				Context("after VM has been manually deleted", func() {
					BeforeEach(func() {
						// after manual deletion (in infrastructure), cloud.HasVM should return false
						expectHasVM1.Return(false, nil)
					})

					It("migrates the disk content, but does not shutdown the old VM", func() {
						expectDeployWithDiskMigrationNoVMShutdown()

						err := newDeployCmd().Run([]string{"/fake-cpi-release.tgz", "/fake-stemcell-release.tgz"})
						Expect(err).ToNot(HaveOccurred())
					})
				})

				Context("after migration has failed", func() {
					BeforeEach(func() {
						expectDeployWithDiskMigrationFailure()

						err := newDeployCmd().Run([]string{"/fake-cpi-release.tgz", "/fake-stemcell-release.tgz"})
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("fake-migration-error"))

						diskRecords, err := diskRepo.All()
						Expect(err).ToNot(HaveOccurred())
						Expect(diskRecords).To(HaveLen(2)) // current + unused

						// reset output buffer
						ui.Said = []string{}

						mockCloud.EXPECT().HasVM("fake-vm-cid-2").Return(true, nil)
					})

					It("deletes unused disks", func() {
						expectDeployWithDiskMigrationRepair()

						mockCloud.EXPECT().DeleteDisk("fake-disk-cid-2")

						err := newDeployCmd().Run([]string{"/fake-cpi-release.tgz", "/fake-stemcell-release.tgz"})
						Expect(err).ToNot(HaveOccurred())

						diskRecord, found, err := diskRepo.FindCurrent()
						Expect(err).ToNot(HaveOccurred())
						Expect(found).To(BeTrue())
						Expect(diskRecord.CID).To(Equal("fake-disk-cid-3"))

						diskRecords, err := diskRepo.All()
						Expect(err).ToNot(HaveOccurred())
						Expect(diskRecords).To(Equal([]bmconfig.DiskRecord{diskRecord}))
					})
				})
			})
		})

		Context("when the registry is configured", func() {
			It("makes the registry available for all CPI commands", func() {
				expectDeployFlowWithRegistry()

				err := newDeployCmd().Run([]string{"/fake-cpi-release.tgz", "/fake-stemcell-release.tgz"})
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})