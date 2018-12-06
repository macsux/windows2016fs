package windows2016fs_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

func expectCommand(executable string, params ...string) {
	command := exec.Command(executable, params...)
	session, err := Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	Eventually(session, 10*time.Minute).Should(Exit(0))
}

func buildDockerImage(tempDirPath, imageId, tag string) {
	dockerSrcPath := filepath.Join(tag, "Dockerfile")
	Expect(dockerSrcPath).To(BeARegularFile())

	depDir := os.Getenv("DEPENDENCIES_DIR")
	Expect(depDir).To(BeADirectory())

	expectCommand("powershell", "Copy-Item", "-Path", dockerSrcPath, "-Destination", tempDirPath)

	expectCommand("powershell", "Copy-Item", "-Path", filepath.Join(depDir, "*"), "-Destination", tempDirPath)

	expectCommand("docker", "build", "-f", filepath.Join(tempDirPath, "Dockerfile"), "--tag", imageId, tempDirPath)
}

func expectMountSMBImage(shareUnc, shareUsername, sharePassword, tempDirPath, imageId string) {
	command := exec.Command(
		"docker",
		"run",
		"--rm",
		"--interactive",
		"--env", fmt.Sprintf("SHARE_UNC=%s", shareUnc),
		"--env", fmt.Sprintf("SHARE_USERNAME=%s", shareUsername),
		"--env", fmt.Sprintf("SHARE_PASSWORD=%s", sharePassword),
		imageId,
		"powershell",
	)

	stdin, err := command.StdinPipe()
	Expect(err).ToNot(HaveOccurred())

	session, err := Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	containerTestPs1Content, err := ioutil.ReadFile("container-test.ps1")
	Expect(err).ToNot(HaveOccurred())

	_, err = io.WriteString(stdin, string(containerTestPs1Content))
	Expect(err).ToNot(HaveOccurred())
	stdin.Close()

	Expect(err).ToNot(HaveOccurred())
	Eventually(session, 5*time.Minute).Should(Exit(0))
}

var _ = Describe("Windows2016fs", func() {
	var (
		tag           string
		imageId       string
		tempDirPath   string
		shareUsername string
		sharePassword string
		shareName     string
		err           error
	)

	BeforeSuite(func() {
		tag = os.Getenv("VERSION_TAG")
		imageId = fmt.Sprintf("windows2016fs-ci:%s", tag)
		tempDirPath, err = ioutil.TempDir("", "build")
		shareName = os.Getenv("SHARE_NAME")
		shareUsername = os.Getenv("SHARE_USERNAME")
		sharePassword = os.Getenv("SHARE_PASSWORD")

		buildDockerImage(tempDirPath, imageId, tag)
	})

	It("can write to an IP-based smb share", func() {
		shareIP := os.Getenv("SHARE_IP")
		shareUnc := fmt.Sprintf("\\\\%s\\%s", shareIP, shareName)
		expectMountSMBImage(shareUnc, shareUsername, sharePassword, tempDirPath, imageId)
	})

	It("can write to an FQDN-based smb share", func() {
		shareFqdn := os.Getenv("SHARE_FQDN")
		shareUnc := fmt.Sprintf("\\\\%s\\%s", shareFqdn, shareName)
		expectMountSMBImage(shareUnc, shareUsername, sharePassword, tempDirPath, imageId)
	})
})
