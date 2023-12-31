package action

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devsolux/go-check/common/batch"
	"github.com/devsolux/go-check/executor"

	"github.com/AlecAivazis/survey/v2"
	"github.com/avast/retry-go/v4"
	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

type mod module.Version

func (m mod) String() string {
	return m.Path + "@" + m.Version
}

func Contains[S ~[]E, E comparable](s S, v E) bool {
	for _, item := range s {
		if item == v {
			return true
		}
	}
	return false
}

func Upgrade(cmd *cobra.Command, args []string) {
	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Color("yellow")

	pwd, err := os.Getwd()
	if err != nil {
		println(err.Error())
		return
	}

	modFile := filepath.Join(pwd, "go.mod")
	modBuf, err := os.ReadFile(modFile)
	if err != nil {
		println("❌ get go.mod failed:", err.Error())
		return
	}
	f, err := modfile.Parse(modFile, modBuf, nil)
	if err != nil {
		println(err.Error())
		return
	}

	directoryModulePath := []string{}
	for _, require := range f.Replace {
		if modfile.IsDirectoryPath(require.New.Path) {
			directoryModulePath = append(directoryModulePath, require.Old.Path)
		}
	}

	mainModule := []mod{}
	for _, require := range f.Require {
		if require.Indirect {
			continue
		}

		if Contains(directoryModulePath, require.Mod.Path) {
			println("🚧", require.Mod.Path, "is replace as a directory path, skip it.")
			continue
		}

		mainModule = append(mainModule, mod(require.Mod))
	}

	println("🔍 find main module:\n")
	for _, m := range mainModule {
		println(m.String())
	}
	println("")

	s.Suffix = " find module information..."
	s.Start()

	b, _ := batch.New(context.Background(), batch.WithConcurrencyNum(10))
	for _, module := range mainModule {
		m := module
		b.Go(m.Path, func() (ret interface{}, err error) {
			err = retry.Do(
				func() error {
					info, err := executor.GetModuleUpdate(pwd, m.Path)
					if err == nil {
						ret = info
					}
					return err
				},
			)
			return
		})
	}

	result, bErr := b.WaitAndGetResult()
	s.Stop()
	if bErr != nil {
		println(bErr.Err.Error())
		return
	}

	texts := []string{}
	mapping := map[string]*executor.Module{}
	for _, require := range f.Require {
		if require.Indirect {
			continue
		}

		value, ok := result[require.Mod.Path]
		if !ok {
			continue
		}

		pkg := value.Value.(*executor.Module)

		if pkg.Update == nil {
			continue
		}

		if pkg.Update.Version != require.Mod.Version {
			key := fmt.Sprintf("%s (%s --> %s)", pkg.Path, require.Mod.Version, pkg.Update.Version)
			texts = append(texts, key)
			mapping[key] = pkg
		}
	}

	if len(texts) == 0 {
		println("🎉  Your modules look amazing. Keep up the great work.")
		return
	}

	selected := []string{}
	prompt := &survey.MultiSelect{
		Message:  "Select the packages you want to upgrade",
		PageSize: 20,
		Options:  texts,
	}
	survey.AskOne(prompt, &selected)

	if len(selected) == 0 {
		return
	}

	answer := "\n\n" + strings.Join(selected, "\n") + "\n"
	prompt.Render(
		survey.MultiSelectQuestionTemplate,
		survey.MultiSelectTemplateData{
			MultiSelect: *prompt,
			Answer:      answer,
			ShowAnswer:  true,
			Config:      &survey.PromptConfig{},
		},
	)

	shouldUpgrade := []*executor.Module{}
	for _, item := range selected {
		shouldUpgrade = append(shouldUpgrade, mapping[item])
	}

	s.Suffix = " Installing using `go get`..."
	s.Start()
	_, err = executor.Upgrade(pwd, shouldUpgrade)
	s.Stop()
	if err != nil {
		println(err.Error())
		return
	}

	s.Suffix = " Running `go mod tidy`"
	s.Start()
	_, err = executor.Tidy(pwd)
	s.Stop()
	if err != nil {
		println(err.Error())
		return
	}

	println("🎉  Update complete!")
}
