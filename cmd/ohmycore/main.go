package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{Use: "ohmycore"}
	root.AddCommand(&cobra.Command{Use: "version", Run: func(cmd *cobra.Command, args []string) { fmt.Println("ohmycore") }})
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
