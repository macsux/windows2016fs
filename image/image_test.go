package image_test

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/windows2016fs/image"
	"code.cloudfoundry.org/windows2016fs/image/imagefakes"
	"code.cloudfoundry.org/windows2016fs/layer"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	"github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("Image", func() {
	var (
		srcDir   string
		destDir  string
		tempDir  string
		manifest v1.Manifest
		config   v1.Image
		lm       *imagefakes.FakeLayerManager
		im       *image.Manager
	)

	const (
		layer1gzip = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		layer2gzip = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		layer3gzip = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

		layer1diffId = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
		layer2diffId = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
		layer3diffId = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	)

	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "windows2016fs.image")
		Expect(err).NotTo(HaveOccurred())

		srcDir = filepath.Join(tempDir, "src")
		destDir = filepath.Join(tempDir, "dest")

		diffIds := []digest.Digest{
			digest.NewDigestFromEncoded("sha256", layer1diffId),
			digest.NewDigestFromEncoded("sha256", layer2diffId),
			digest.NewDigestFromEncoded("sha256", layer3diffId),
		}

		config = v1.Image{
			Architecture: "amd64",
			OS:           "windows",
			RootFS:       v1.RootFS{Type: "layers", DiffIDs: diffIds},
		}
		cdesc := writeBlob(srcDir, config)

		layers := []v1.Descriptor{
			{Digest: digest.NewDigestFromEncoded("sha256", layer1), MediaType: "some.type.tar.gzip"},
			{Digest: digest.NewDigestFromEncoded("sha256", layer2), MediaType: "some.other.type.tar+gzip"},
			{Digest: digest.NewDigestFromEncoded("sha256", layer3), MediaType: "some.type.tar.gzip"},
		}

		manifest = v1.Manifest{
			Config: cdesc,
			Layers: layers,
		}
		mdesc := writeBlob(srcDir, manifest)
		writeIndexJson(srcDir, mdesc)

		lm = &imagefakes.FakeLayerManager{}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tempDir)).To(Succeed())
	})

	JustBeforeEach(func() {
		im = image.NewManager(srcDir, lm, ioutil.Discard)
	})

	FDescribe("LoadMetadata", func() {
		It("loads the manifest and the config from the image directory", func() {
			Expect(im.LoadMetadata()).To(Succeed())
			Expect(im.manifest)
		})
	})

	Describe("Extract", func() {
		It("extracts all the layers, returning the top layer id", func() {
			topLayerId, err := im.Extract()
			Expect(err).NotTo(HaveOccurred())
			Expect(topLayerId).To(Equal(layer3))

			Expect(lm.DeleteCallCount()).To(Equal(0))

			Expect(lm.ExtractCallCount()).To(Equal(3))
			tgz, id, parentIds := lm.ExtractArgsForCall(0)
			Expect(tgz).To(Equal(filepath.Join(srcDir, layer1)))
			Expect(id).To(Equal(layer1))
			Expect(parentIds).To(Equal([]string{}))

			tgz, id, parentIds = lm.ExtractArgsForCall(1)
			Expect(tgz).To(Equal(filepath.Join(srcDir, layer2)))
			Expect(id).To(Equal(layer2))
			Expect(parentIds).To(Equal([]string{layer1}))

			tgz, id, parentIds = lm.ExtractArgsForCall(2)
			Expect(tgz).To(Equal(filepath.Join(srcDir, layer3)))
			Expect(id).To(Equal(layer3))
			Expect(parentIds).To(Equal([]string{layer2, layer1}))
		})

		Context("a layer has already been extracted", func() {
			BeforeEach(func() {
				lm.StateReturnsOnCall(1, layer.Valid, nil)
			})

			It("does not re-extract the existing layer", func() {
				_, err := im.Extract()
				Expect(err).NotTo(HaveOccurred())
				Expect(lm.DeleteCallCount()).To(Equal(0))

				Expect(lm.ExtractCallCount()).To(Equal(2))
				tgz, id, parentIds := lm.ExtractArgsForCall(0)
				Expect(tgz).To(Equal(filepath.Join(srcDir, layer1)))
				Expect(id).To(Equal(layer1))
				Expect(parentIds).To(Equal([]string{}))

				tgz, id, parentIds = lm.ExtractArgsForCall(1)
				Expect(tgz).To(Equal(filepath.Join(srcDir, layer3)))
				Expect(id).To(Equal(layer3))
				Expect(parentIds).To(Equal([]string{layer2, layer1}))
			})
		})

		Context("there is an invalid layer", func() {
			BeforeEach(func() {
				lm.StateReturnsOnCall(1, layer.Incomplete, nil)
			})

			It("deletes the incomplete layer and re-extracts", func() {
				_, err := im.Extract()
				Expect(err).NotTo(HaveOccurred())

				Expect(lm.DeleteCallCount()).To(Equal(1))
				Expect(lm.DeleteArgsForCall(0)).To(Equal(layer2))

				Expect(lm.ExtractCallCount()).To(Equal(3))
				tgz, id, parentIds := lm.ExtractArgsForCall(0)
				Expect(tgz).To(Equal(filepath.Join(srcDir, layer1)))
				Expect(id).To(Equal(layer1))
				Expect(parentIds).To(Equal([]string{}))

				tgz, id, parentIds = lm.ExtractArgsForCall(1)
				Expect(tgz).To(Equal(filepath.Join(srcDir, layer2)))
				Expect(id).To(Equal(layer2))
				Expect(parentIds).To(Equal([]string{layer1}))

				tgz, id, parentIds = lm.ExtractArgsForCall(2)
				Expect(tgz).To(Equal(filepath.Join(srcDir, layer3)))
				Expect(id).To(Equal(layer3))
				Expect(parentIds).To(Equal([]string{layer2, layer1}))
			})
		})

		Context("provided an invalid content digest", func() {
			BeforeEach(func() {
				manifest = v1.Manifest{
					Layers: []v1.Descriptor{
						{Digest: digest.Digest("hello"), MediaType: "something.tar.gzip"},
					},
				}
			})

			It("returns an error", func() {
				_, err := im.Extract()
				Expect(err).To(MatchError(digest.ErrDigestInvalidFormat))
			})
		})

		Context("the media type is not a .tar.gzip or .tar+gzip", func() {
			BeforeEach(func() {
				manifest = v1.Manifest{
					Layers: []v1.Descriptor{
						{Digest: digest.NewDigestFromEncoded("sha256", layer1), MediaType: "some-invalid-string"},
					},
				}
			})

			It("returns an error", func() {
				_, err := im.Extract()
				Expect(err).To(MatchError(errors.New("invalid layer media type: some-invalid-string")))
			})
		})
	})
})

func (m *Metadata) Write() error {
	if err := m.writeOCILayout(); err != nil {
		return err
	}

	configDescriptor, err := m.writeConfig()
	if err != nil {
		return err
	}

	manifestDescriptor, err := m.writeManifest(configDescriptor)
	if err != nil {
		return err
	}

	return m.writeIndexJson(manifestDescriptor)
}

func (m *Metadata) writeOCILayout() error {
	il := v1.ImageLayout{
		Version: specs.Version,
	}
	data, err := json.Marshal(il)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(m.outDir, "oci-layout"), data, 0644)
}

func writeBlob(outDir string, blob interface{}) v1.Descriptor {
	data, err := json.Marshal(blob)
	Expect(err).NotTo(HaveOccurred())

	blobSha := fmt.Sprintf("%x", sha256.Sum256(data))

	blobsDir := filepath.Join(outDir, "blobs", "sha256")
	Expect(os.MkdirAll(blobsDir, 0755)).To(Succeed())

	Expect(ioutil.WriteFile(filepath.Join(blobsDir, blobSha), data, 0644)).To(Succeed())

	return v1.Descriptor{
		Size:   int64(len(data)),
		Digest: digest.NewDigestFromEncoded(digest.SHA256, blobSha),
	}
}

func writeConfig(outDir string, diffIds []digest.Digest) v1.Descriptor {
	ic := v1.Image{
		Architecture: "amd64",
		OS:           "windows",
		RootFS:       v1.RootFS{Type: "layers", DiffIDs: diffIds},
	}

	d, err := m.writeBlob(ic)
	Expect(err).NotTo(HaveOccurred())

	d.MediaType = v1.MediaTypeImageConfig
	return d
}

func (m *Metadata) writeManifest(config v1.Descriptor, layers []v1.Descriptor) v1.Descriptor {
	im := v1.Manifest{
		Versioned: specs.Versioned{SchemaVersion: 2},
		Config:    config,
		Layers:    layers,
	}

	d, err := m.writeBlob(im)
	Expect(err).NotTo(HaveOccurred())

	d.MediaType = v1.MediaTypeImageManifest
	d.Platform = &v1.Platform{OS: "windows", Architecture: "amd64"}
	return d
}

func writeIndexJson(outDir string, manifest v1.Descriptor) {
	ii := v1.Index{
		Versioned: specs.Versioned{SchemaVersion: 2},
		Manifests: []v1.Descriptor{manifest},
	}

	data, err := json.Marshal(ii)
	Expect(err).NotTo(HaveOccurred())
	Expect(ioutil.WriteFile(filepath.Join(outDir, "index.json"), data, 0644)).To(Succeed())
}
