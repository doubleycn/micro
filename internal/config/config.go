package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	conf "github.com/micro/go-micro/v2/config"
	"github.com/micro/go-micro/v2/config/source/file"
	"github.com/micro/go-micro/v2/util/log"

	"github.com/gofrs/flock"
)

// FileName for global micro config
const FileName = ".micro"

// config is a singleton which is required to ensure
// each function call doesn't load the .micro file
// from disk
var config = newConfig()
var errs = []string{}

type lockedConfig struct {
	c conf.Config
	m *flock.Flock
}

// Get a value from the .micro file
func Get(path ...string) (string, error) {
	tk := config.c.Get(path...).String("")
	return strings.TrimSpace(tk), nil
}

// Set a value in the .micro file
func Set(value string, path ...string) error {
	// get the filepath
	fp, err := filePath()
	if err != nil {
		return err
	}
	// set the value
	config.c.Set(value, path...)

	// write to the file
	b := config.c.Bytes()
	if len(b) == 0 {
		errs = append(errs, fmt.Sprintf("Tried to set zero config value %s, path %s", value, path))
		// return errors.New("Trying to set config but bytes is empty")
	}
	if err := ioutil.WriteFile(fp, b, 0644); err != nil {
		errs = append(errs, err.Error())
	}
	return err

}

func Errors() []string {
	return errs
}

func Lock() error {
	err := config.m.Lock()
	if err != nil {
		return err
	}
	return config.c.Sync()
}

func Unlock() error {
	return config.m.Unlock()
}

func filePath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(usr.HomeDir, FileName), nil
}

// newConfig returns a loaded config
func newConfig() *lockedConfig {
	// get the filepath
	fp, err := filePath()
	if err != nil {
		log.Error(err)
		return &lockedConfig{c: conf.DefaultConfig, m: flock.NewFlock(os.TempDir() + FileName + ".lock")}
	}
	m := flock.NewFlock(fp + ".lock")
	m.Lock()
	defer m.Unlock()

	// write the file if it does not exist
	// this is the problem
	if _, err := os.Stat(fp); os.IsNotExist(err) {
		ioutil.WriteFile(fp, []byte{}, 0644)
	} else if err != nil {
		errs = append(errs, "OS stat err: "+err.Error())
		log.Error(err)
		return &lockedConfig{c: conf.DefaultConfig, m: m}
	}

	// create a new config
	c, err := conf.NewConfig(
		conf.WithSource(
			file.NewSource(
				file.WithPath(fp),
			),
		),
	)
	if err != nil {
		log.Error(err)
		return &lockedConfig{c: conf.DefaultConfig, m: m}
	}

	// load the config
	if err := c.Load(); err != nil {
		log.Error(err)
		return &lockedConfig{c: conf.DefaultConfig, m: m}
	}

	// return the conf
	return &lockedConfig{c: c, m: m}

}
