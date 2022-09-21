// FIXME: golangci-lint
// nolint:errcheck,revive
package services_test

import (
	"archive/tar"
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services"
	log "github.com/sirupsen/logrus"
)

var testFile = "test.txt"
var testTarFile = "test.tar"

var _ = Describe("RepoBuilder Service Test", func() {
	var service services.RepoBuilderInterface
	BeforeEach(func() {
		var ctx context.Context = context.Background()

		ctrl := gomock.NewController(GinkgoT())
		defer ctrl.Finish()
		service = services.NewRepoBuilder(ctx, log.NewEntry(log.StandardLogger()))
	})
	Describe("#ExtractVersionRepo", func() {
		When("is valid", func() {
			It("should extract the tar file", func() {
				commit := &models.Commit{}

				filePath := fmt.Sprintf("/tmp/tar_extract_test_%d", time.Now().Unix())
				filePathExtraction := filepath.Join(filePath, filePath)

				os.MkdirAll(filePathExtraction, 0755)
				testFilePath, _ := createTestFile(filePath)
				testTarFile = filepath.Join(filePath, testTarFile)
				err := createTarball(testTarFile, testFilePath)

				Expect(err).ToNot(HaveOccurred())

				err = service.ExtractVersionRepo(commit, testTarFile, filePath)

				Expect(err).ToNot(HaveOccurred())

				fileContent, err := readTestFile(filePathExtraction)
				Expect(err).ToNot(HaveOccurred())
				Expect(fileContent).To(Equal("Some content to test"))
			})
		})
	})
})

func createTarball(tarballFilePath string, filePath string) error {
	file, err := os.Create(tarballFilePath)
	if err != nil {
		return fmt.Errorf("Could not create tarball file '%s', got error '%s'", tarballFilePath, err.Error())
	}
	defer file.Close()

	tarWriter := tar.NewWriter(file)
	defer tarWriter.Close()

	err = addFileToTarWriter(filePath, tarWriter)
	if err != nil {
		return fmt.Errorf("Could not add file '%s', to tarball, got error '%s'", filePath, err.Error())
	}

	return nil
}

func addFileToTarWriter(filePath string, tarWriter *tar.Writer) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("Could not open file '%s', got error '%s'", filePath, err.Error())
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("Could not get stat for file '%s', got error '%s'", filePath, err.Error())
	}

	header := &tar.Header{
		Name:    filePath,
		Size:    stat.Size(),
		Mode:    int64(stat.Mode()),
		ModTime: stat.ModTime(),
	}

	err = tarWriter.WriteHeader(header)
	if err != nil {
		return fmt.Errorf("Could not write header for file '%s', got error '%s'", filePath, err.Error())
	}

	_, err = io.Copy(tarWriter, file)
	if err != nil {
		return fmt.Errorf("Could not copy the file '%s' data to the tarball, got error '%s'", filePath, err.Error())
	}

	return nil
}

func createTestFile(filePath string) (string, error) {
	testFilePath := filepath.Join(filePath, testFile)
	f, err := os.Create(testFilePath)
	if err != nil {
		return "", fmt.Errorf("Could not create the file on '%s', got error '%s'", filePath, err.Error())
	}
	defer f.Close()

	_, err = f.WriteString("Some content to test")
	if err != nil {
		return "", fmt.Errorf("Could not write on the file '%s', got error '%s'", filePath, err.Error())
	}

	return testFilePath, nil
}

func readTestFile(filePath string) (string, error) {
	testFilePath := filepath.Join(filePath, testFile)
	f, err := os.Open(testFilePath)
	if err != nil {
		return "", fmt.Errorf("Could not open the file on '%s', got error '%s'", filePath, err.Error())
	}
	defer f.Close()

	r := bufio.NewReader(f)
	fileContent, _, err := r.ReadLine()
	if err != nil {
		return "", fmt.Errorf("Could not read the file '%s', got error '%s'", filePath, err.Error())
	}

	return string(fileContent), nil
}
