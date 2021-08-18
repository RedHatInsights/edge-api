package services

import (
	"archive/tar"
	"log"
	"os"
	"testing"
)

func TestUntar(t *testing.T) {
	//create tar file to be used as mock
	tarPath := "mockTarFile.tar"
	files := map[string]string{
		"index.html":   `<body>Ansible!</body>`,
		"lang.json":    `[{"code":"pt","name":"{Portuguese}"}]`,
		"mock_txt.txt": `some content about red hat`,
	}
	tarWrite := func(data map[string]string) error {
		tarFile, err := os.Create(tarPath)
		if err != nil {
			log.Fatal(err)
		}
		defer tarFile.Close()
		tw := tar.NewWriter(tarFile)
		defer tw.Close()
		for name, content := range data {
			hdr := &tar.Header{
				Name: name,
				Mode: 0600,
				Size: int64(len(content)),
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if _, err := tw.Write([]byte(content)); err != nil {
				return err
			}
		}
		return nil
	}
	if err := tarWrite(files); err != nil {
		log.Fatal(err)
	}
	unTarFile, errOpenFile := os.Open(tarPath)
	if errOpenFile != nil {
		t.Error("Unable to open mock tar file before test")
	}
	NewExtractor().Extract(unTarFile, `./`)
	for name := range files {
		// check if file exist after untar method calls
		if _, err := os.Stat(name); os.IsNotExist(err) {
			t.Fail()
		}
		os.Remove(name)
	}
	os.Remove(tarPath)
}
