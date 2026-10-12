package server_test

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/onsi/disco/server"
)

var _ = Describe("TemplateRenderer", func() {
	var rootPath string

	write := func(path, contents string) {
		GinkgoHelper()
		full := filepath.Join(rootPath, path)
		Ω(os.MkdirAll(filepath.Dir(full), 0o755)).Should(Succeed())
		Ω(os.WriteFile(full, []byte(contents), 0o644)).Should(Succeed())
	}

	BeforeEach(func() {
		rootPath = GinkgoT().TempDir() + "/"
		write("js/app.js", "console.log('hi')\n")
	})

	Describe("when a template fails to parse", func() {
		BeforeEach(func() {
			write("templates/broken.html", `{{ define "broken" }}{{ end `)
		})

		It("returns the error each time instead of wedging later renders", func(ctx SpecContext) {
			renderer := NewTemplateRenderer(rootPath, false)
			done := make(chan error, 2)
			go func() {
				defer GinkgoRecover()
				done <- renderer.Render(&bytes.Buffer{}, "broken", nil, nil)
				done <- renderer.Render(&bytes.Buffer{}, "broken", nil, nil)
			}()
			for i := 0; i < 2; i++ {
				select {
				case err := <-done:
					Ω(err).Should(HaveOccurred())
				case <-ctx.Done():
					Fail("Render never returned: the renderer lock was not released")
				}
			}
		}, SpecTimeout(5*time.Second))
	})

	Describe("building assets in production mode", func() {
		BeforeEach(func() {
			write("templates/page.html", `{{ define "page" }}{{ build "js/app.js" "script" }}{{ end }}`)
		})

		It("is safe to render concurrently", func() {
			renderer := NewTemplateRenderer(rootPath, false)
			wg := &sync.WaitGroup{}
			for i := 0; i < 32; i++ {
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					buf := &bytes.Buffer{}
					Ω(renderer.Render(buf, "page", nil, nil)).Should(Succeed())
					Ω(buf.String()).Should(ContainSubstring("console.log"))
				}()
			}
			wg.Wait()
		})
	})
})
