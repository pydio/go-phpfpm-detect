package fpm

import (
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/kardianos/osext"
	. "github.com/smartystreets/goconvey/convey"
)

func TestDetectPhpFpm(t *testing.T) {

	Convey("Test combined detection", t, func() {

		config, e := DetectFpmInfos()
		So(e, ShouldBeNil)
		So(config.ListenAddress, ShouldNotBeEmpty)

	})

}

func TestDetectByDirectConnection(t *testing.T) {

	Convey("Direct connection to PHP?", t, func() {

		config := &PhpFpmConfig{}
		e := detectByDirectConnection(config)
		So(e, ShouldBeNil)

	})
}

func TestDetectPhpInfos(t *testing.T) {

	Convey("Detect Php Infos", t, func() {

		c := &PhpFpmConfig{ListenNetwork: "tcp", ListenAddress: "127.0.0.1:9000"}

		ex, _ := osext.ExecutableFolder()
		e := DetectPhpInfos(c, ex)
		So(e, ShouldBeNil)
		So(c.PhpVersion, ShouldNotBeNil)
		So(c.PhpExtensions, ShouldNotBeNil)
		compare, _ := version.NewVersion("7.0")
		detected, _ := version.NewVersion(c.PhpVersion)
		So(detected.GreaterThan(compare), ShouldBeTrue)
		So(c.PhpExtensions, ShouldContain, "Core")

	})

}

// GoPath returns the current GOPATH env var
func GoPath() string {
	go_path := strings.Split(os.Getenv("GOPATH"), string(os.PathListSeparator))
	return go_path[0]
}
