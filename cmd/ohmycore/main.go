package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"ohmycore/core"
	"ohmycore/internal/links"
	"ohmycore/internal/mihomo"
	"ohmycore/internal/xray"
)

var baseDir string

func newManager() *core.Manager {
	m := core.NewManager(baseDir)
	m.RegisterAdapter(core.KernelXray, xray.New())
	m.RegisterAdapter(core.KernelMihomo, mihomo.New())
	return m
}

func main() {
	root := &cobra.Command{Use: "ohmycore"}
	root.PersistentFlags().StringVar(&baseDir, "base-dir", defaultBaseDir(), "base directory")
	root.AddCommand(instanceCmd())
	root.AddCommand(linkCmd())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func defaultBaseDir() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".ohmycore")
}

func instanceCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "instance"}
	create := &cobra.Command{
		Use: "create",
		RunE: func(cmd *cobra.Command, args []string) error {
			kernel, _ := cmd.Flags().GetString("kernel")
			cfgPath, _ := cmd.Flags().GetString("config")
			cfg, err := os.ReadFile(cfgPath)
			if err != nil {
				return err
			}
			k := core.Kernel(kernel)
			m := newManager()
			inst, err := m.Create(k, cfg)
			if err != nil {
				return err
			}
			fmt.Println(inst.ID)
			return nil
		},
	}
	create.Flags().String("kernel", "xray", "kernel")
	create.Flags().String("config", "", "config file")
	cmd.AddCommand(create)

	start := &cobra.Command{
		Use: "start [id]",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := newManager()
			return m.Start(args[0])
		},
	}
	cmd.AddCommand(start)

	stop := &cobra.Command{
		Use: "stop [id]",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := newManager()
			return m.Stop(args[0])
		},
	}
	cmd.AddCommand(stop)

	remove := &cobra.Command{
		Use: "remove [id]",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := newManager()
			return m.Remove(args[0])
		},
	}
	cmd.AddCommand(remove)

	list := &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := newManager()
			for _, inst := range m.List() {
				b, _ := json.Marshal(inst)
				fmt.Println(string(b))
			}
			return nil
		},
	}
	cmd.AddCommand(list)

	status := &cobra.Command{
		Use: "status [id]",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := newManager()
			inst, err := m.Get(args[0])
			if err != nil {
				return err
			}
			b, _ := json.Marshal(inst)
			fmt.Println(string(b))
			return nil
		},
	}
	cmd.AddCommand(status)
	return cmd
}

func linkCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "link"}
	check := &cobra.Command{
		Use: "check [link]",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kernel, _ := cmd.Flags().GetString("kernel")
			link := args[0]
			if kernel == "xray" || kernel == "both" {
				r, err := links.CheckXrayLinkSupport(link)
				if err != nil {
					return err
				}
				b, _ := json.Marshal(r)
				fmt.Println("xray:", string(b))
			}
			if kernel == "mihomo" || kernel == "both" {
				r, err := links.CheckMihomoLinkSupport(link)
				if err != nil {
					return err
				}
				b, _ := json.Marshal(r)
				fmt.Println("mihomo:", string(b))
			}
			return nil
		},
	}
	check.Flags().String("kernel", "both", "kernel")
	cmd.AddCommand(check)

	convert := &cobra.Command{
		Use: "convert [link]",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kernel, _ := cmd.Flags().GetString("kernel")
			link := args[0]
			if kernel == "xray" {
				b, err := links.LinkToXrayConfig(link)
				if err != nil {
					return err
				}
				fmt.Println(string(b))
			} else {
				b, err := links.LinkToMihomoConfig(link)
				if err != nil {
					return err
				}
				fmt.Println(string(b))
			}
			return nil
		},
	}
	convert.Flags().String("kernel", "xray", "kernel")
	cmd.AddCommand(convert)
	return cmd
}
