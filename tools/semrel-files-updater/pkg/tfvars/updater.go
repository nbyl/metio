package tfvars

import (
	"os"
	"regexp"
)

var FUVERSION = "dev"

type Updater struct{}

func (u *Updater) Init(_ map[string]string) error {
	return nil
}

func (u *Updater) Name() string {
	return "tfvars"
}

func (u *Updater) Version() string {
	return FUVERSION
}

func (u *Updater) ForFiles() string {
	return `variables\.tf`
}

func (u *Updater) Apply(file, newVersion string) error {
	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	re := regexp.MustCompile(`:0\.0\.0`)
	newContent := re.ReplaceAll(content, []byte(":"+newVersion))

	return os.WriteFile(file, newContent, 0644)
}
