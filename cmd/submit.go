package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/exercism/cli/api"
	"github.com/exercism/cli/config"
	"github.com/exercism/cli/workspace"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// submitCmd lets people upload a solution to the website.
var submitCmd = &cobra.Command{
	Use:     "submit",
	Aliases: []string{"s"},
	Short:   "Submit your solution to an exercise.",
	Long: `Submit your solution to an Exercism exercise.

The CLI will do its best to figure out what to submit.

If you call the command without any arguments, it will
submit the exercise contained in the current directory.

If called with the path to a directory, it will submit it.

If called with the name of an exercise, it will work out which
track it is on and submit it. The command will ask for help
figuring things out if necessary.
`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.NewConfiguration()

		usrCfg := viper.New()
		usrCfg.AddConfigPath(cfg.Dir)
		usrCfg.SetConfigName("user")
		usrCfg.SetConfigType("json")
		// Ignore error. If the file doesn't exist, that is fine.
		_ = usrCfg.ReadInConfig()
		cfg.UserViperConfig = usrCfg

		v := viper.New()
		v.AddConfigPath(cfg.Dir)
		v.SetConfigName("cli")
		v.SetConfigType("json")
		// Ignore error. If the file doesn't exist, that is fine.
		_ = v.ReadInConfig()

		return runSubmit(cfg, cmd.Flags(), args)
	},
}

func runSubmit(cfg config.Configuration, flags *pflag.FlagSet, args []string) error {
	usrCfg := cfg.UserViperConfig

	if usrCfg.GetString("token") == "" {
		return errors.New("TODO: Welcome to Exercism this is how you use this")
	}

	if usrCfg.GetString("workspace") == "" {
		return errors.New("TODO: run configure first")
	}

	for i, arg := range args {
		info, err := os.Lstat(arg)
		if err != nil {
			if os.IsNotExist(err) {
				return errors.New("TODO: explain that there is no such file")
			}
			return err
		}
		if info.IsDir() {
			return errors.New("TODO: it is a directory and we cannot handle that")
		}

		src, err := filepath.EvalSymlinks(arg)
		if err != nil {
			return err
		}
		args[i] = src
	}

	ws, err := workspace.New(usrCfg.GetString("workspace"))
	if err != nil {
		return err
	}

	tx, err := workspace.NewTransmission(ws.Dir, args)
	if err != nil {
		return err
	}

	dirs, err := ws.Locate(tx.Dir)
	if err != nil {
		return err
	}

	sx, err := workspace.NewSolutions(dirs)
	if err != nil {
		return err
	}
	if len(sx) == 0 {
		// TODO: add test
		return errors.New("can't find a solution metadata file. (todo: explain how to fix it)")
	}
	if len(sx) > 1 {
		return errors.New("files from multiple solutions. Can only submit one solution at a time. (todo: fix error message)")
	}
	solution := sx[0]

	if !solution.IsRequester {
		// TODO: add test
		return errors.New("not your solution. todo: fix error message")
	}

	paths := make([]string, 0, len(tx.Files))
	for _, file := range tx.Files {
		// Don't submit empty files
		info, err := os.Stat(file)
		if err != nil {
			return err
		}
		if info.Size() == 0 {
			fmt.Fprintf(Err, "(TODO) Warning: file %s was empty, skipping...", file)
			continue
		}
		paths = append(paths, file)
	}

	if len(paths) == 0 {
		return errors.New("no files found to submit. TODO: fix error messages")
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		dirname := fmt.Sprintf("%s%s%s", string(os.PathSeparator), solution.Exercise, string(os.PathSeparator))
		pieces := strings.Split(path, dirname)
		filename := fmt.Sprintf("%s%s", string(os.PathSeparator), pieces[len(pieces)-1])

		part, err := writer.CreateFormFile("files[]", filename)
		if err != nil {
			return err
		}
		_, err = io.Copy(part, file)
		if err != nil {
			return err
		}
	}

	err = writer.Close()
	if err != nil {
		return err
	}

	client, err := api.NewClient(usrCfg.GetString("token"), usrCfg.GetString("apibaseurl"))
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/solutions/%s", usrCfg.GetString("apibaseurl"), solution.ID)
	req, err := client.NewRequest("PATCH", url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bb := &bytes.Buffer{}
	_, err = bb.ReadFrom(resp.Body)
	if err != nil {
		return err
	}

	if solution.AutoApprove == true {
		msg := `Your solution has been submitted successfully and has been auto-approved.
You can complete the exercise and unlock the next core exercise at:
`
		fmt.Fprintf(Err, msg)
	} else {
		msg := "Your solution has been submitted successfully. View it at:\n"
		fmt.Fprintf(Err, msg)
	}
	fmt.Fprintf(Out, "%s\n", solution.URL)

	return nil
}

func initSubmitCmd() {
	setupSubmitFlags(submitCmd.Flags())
}

func setupSubmitFlags(flags *pflag.FlagSet) {
	flags.StringP("track", "t", "", "the track ID")
	flags.StringP("exercise", "e", "", "the exercise ID")
	flags.StringSliceP("files", "f", make([]string, 0), "files to submit")
}

func init() {
	RootCmd.AddCommand(submitCmd)
}
