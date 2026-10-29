package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
	root.AddCommand(mihomoCmd())
	root.AddCommand(xrayCmd())
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

	for _, name := range []string{"start", "stop", "remove"} {
		n := name
		c := &cobra.Command{
			Use:  n + " [id]",
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				m := newManager()
				switch n {
				case "start":
					return m.Start(args[0])
				case "stop":
					return m.Stop(args[0])
				case "remove":
					return m.Remove(args[0])
				}
				return nil
			},
		}
		cmd.AddCommand(c)
	}

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

func mihomoCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "mihomo"}
	proxies := &cobra.Command{
		Use: "proxies [id]",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := newManager()
			inst, err := m.Get(args[0])
			if err != nil {
				return err
			}
			a, err := mihomoAdapter(m, inst)
			if err != nil {
				return err
			}
			p, err := a.Proxies(inst)
			if err != nil {
				return err
			}
			b, _ := json.Marshal(p)
			fmt.Println(string(b))
			return nil
		},
	}
	cmd.AddCommand(proxies)

	delay := &cobra.Command{
		Use: "delay [id] [proxy]",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			urlFlag, _ := cmd.Flags().GetString("url")
			timeout, _ := cmd.Flags().GetInt("timeout")
			m := newManager()
			inst, err := m.Get(args[0])
			if err != nil {
				return err
			}
			a, err := mihomoAdapter(m, inst)
			if err != nil {
				return err
			}
			d, err := a.Delay(inst, args[1], urlFlag, time.Duration(timeout)*time.Millisecond)
			if err != nil {
				return err
			}
			fmt.Println(d)
			return nil
		},
	}
	delay.Flags().String("url", "http://www.gstatic.com/generate_204", "test url")
	delay.Flags().Int("timeout", 5000, "timeout ms")
	cmd.AddCommand(delay)

	selectCmd := &cobra.Command{
		Use: "select [id] [group] [proxy]",
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := newManager()
			inst, err := m.Get(args[0])
			if err != nil {
				return err
			}
			a, err := mihomoAdapter(m, inst)
			if err != nil {
				return err
			}
			return a.SelectProxy(inst, args[1], args[2])
		},
	}
	cmd.AddCommand(selectCmd)

	reload := &cobra.Command{
		Use: "reload-providers [id]",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := newManager()
			inst, err := m.Get(args[0])
			if err != nil {
				return err
			}
			a, err := mihomoAdapter(m, inst)
			if err != nil {
				return err
			}
			return a.ReloadProviders(inst)
		},
	}
	cmd.AddCommand(reload)
	return cmd
}

func xrayCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "xray"}
	ping := &cobra.Command{
		Use: "ping [id] [outbound]",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("not implemented")
			return nil
		},
	}
	cmd.AddCommand(ping)
	return cmd
}

func mihomoAdapter(m *core.Manager, inst *core.Instance) (*mihomo.Adapter, error) {
	a, err := m.Adapter(inst.Kernel)
	if err != nil {
		return nil, err
	}
	ma, ok := a.(*mihomo.Adapter)
	if !ok {
		return nil, fmt.Errorf("not mihomo adapter")
	}
	return ma, nil
}
