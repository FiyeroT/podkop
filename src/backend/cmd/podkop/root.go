// The program podkop: the backend of podkop with sing-box inside (podkop-engine, a module),
// and the command line of both.
//
// Installed, or linked, as "sing-box" it is sing-box: the command line of sing-box, with
// the version and the features of podkop-engine. Under any other name it is podkop: its
// own commands (daemon, release, ctl, and in a lab build those of the lab), and the
// commands of the user interface, which the shell backend had as the commands of
// /usr/bin/podkop: "podkop get_status" is "podkop ctl get_status".
package main

import (
	"os"
	"path/filepath"

	"github.com/sagernet/sing-box/cmd/sing-box/cli"

	"github.com/spf13/cobra"
)

var mainCommand = &cobra.Command{
	Use:   "podkop",
	Short: "The backend of podkop: sing-box set up from the settings of podkop (UCI)",
	// the commands of the user interface print what the page reads: cobra adds nothing
	SilenceUsage:      true,
	CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
}

func main() {
	if filepath.Base(os.Args[0]) == "sing-box" {
		cli.Main()
		return
	}
	if len(os.Args) < 2 || !ownCommand(os.Args[1]) {
		os.Args = append([]string{os.Args[0], "ctl"}, os.Args[1:]...)
	}
	if err := mainCommand.Execute(); err != nil {
		os.Exit(1)
	}
}

// ownCommand tells a command of the program from one of the user interface.
func ownCommand(name string) bool {
	for _, command := range mainCommand.Commands() {
		if command.Name() == name {
			return true
		}
	}
	return false
}
