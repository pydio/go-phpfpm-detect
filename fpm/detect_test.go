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

// TestBasics performs basic validation of methods that do not need complex set up nor mocks
func TestProcess(t *testing.T) {

	Convey("PHP is running ?", t, func() {
		p, err := findRunningBinary()

		if err != nil {
			So(err.Error(), ShouldEqual, "not found")
		} else {
			So(p, ShouldNotBeEmpty)
		}
		config := &PhpFpmConfig{}
		_, e := parseCommandConfig(p, config)
		So(e, ShouldBeNil)
		So(config.ListenAddress, ShouldNotBeEmpty)

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
		So(c.PhpVersion.GreaterThan(compare), ShouldBeTrue)
		So(c.PhpExtensions, ShouldContain, "Core")

	})

}

// GoPath returns the current GOPATH env var
func GoPath() string {
	go_path := strings.Split(os.Getenv("GOPATH"), string(os.PathListSeparator))
	return go_path[0]
}
