package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/wagoodman/dive/dive"
	"github.com/wagoodman/dive/dive/filetree"
	"github.com/wagoodman/dive/runtime/ui/components"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var exportFile string
var ciConfigFile string
var ciConfig = viper.New()
var isCi bool

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "dive [IMAGE]",
	Short: "Docker Image Visualizer & Explorer",
	Long: `This tool provides a way to discover and explore the contents of a docker image. Additionally the tool estimates
the amount of wasted space and identifies the offending files from the image.`,
	Args: cobra.MaximumNArgs(1),
	Run:  doAnalyzeCmd,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	initCli()
	cobra.OnInitialize(initConfig)
}

func initCli() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.dive.yaml, ~/.config/dive/*.yaml, or $XDG_CONFIG_HOME/dive.yaml)")
	rootCmd.PersistentFlags().String("source", "docker", "The container engine to fetch the image from. Allowed values: "+strings.Join(dive.ImageSources, ", "))
	rootCmd.PersistentFlags().BoolP("version", "v", false, "display version number")
	rootCmd.PersistentFlags().BoolP("ignore-errors", "i", false, "ignore image parsing errors and run the analysis anyway")
	rootCmd.Flags().BoolVar(&isCi, "ci", false, "Skip the interactive TUI and validate against CI rules (same as env var CI=true)")
	rootCmd.Flags().StringVarP(&exportFile, "json", "j", "", "Skip the interactive TUI and write the layer analysis statistics to a given file.")
	rootCmd.Flags().StringVar(&ciConfigFile, "ci-config", ".dive-ci", "If CI=true in the environment, use the given yaml to drive validation rules.")

	rootCmd.PersistentFlags().BoolP("cnb", "c", false, "show cloud native buildpack app metadata")
	rootCmd.Flags().String("lowestEfficiency", "0.9", "(only valid with --ci given) lowest allowable image efficiency (as a ratio between 0-1), otherwise CI validation will fail.")
	rootCmd.Flags().String("highestWastedBytes", "disabled", "(only valid with --ci given) highest allowable bytes wasted, otherwise CI validation will fail.")
	rootCmd.Flags().String("highestUserWastedPercent", "0.1", "(only valid with --ci given) highest allowable percentage of bytes wasted (as a ratio between 0-1), otherwise CI validation will fail.")

	for _, key := range []string{"lowestEfficiency", "highestWastedBytes", "highestUserWastedPercent"} {
		if err := ciConfig.BindPFlag(fmt.Sprintf("rules.%s", key), rootCmd.Flags().Lookup(key)); err != nil {
			log.Fatalf("Unable to bind '%s' flag: %v", key, err)
		}
	}

	if err := ciConfig.BindPFlag("ignore-errors", rootCmd.PersistentFlags().Lookup("ignore-errors")); err != nil {
		log.Fatalf("Unable to bind 'ignore-errors' flag: %v", err)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	var err error

	viper.SetDefault("log.level", log.InfoLevel.String())
	viper.SetDefault("log.path", "./dive.log")
	viper.SetDefault("log.enabled", false)
	// keybindings: status view / global

	// This keybinding can be changed but ctrl+C may still quit the application
	viper.SetDefault("keybinding.quit", components.NewKeyBinding(
		"Quit",
		tcell.NewEventKey(tcell.KeyCtrlC, rune(0), tcell.ModCtrl),
	))
	viper.SetDefault("keybinding.toggle-view", components.NewKeyBinding(
		"Switch View",
		tcell.NewEventKey(tcell.KeyTab, rune(0), tcell.ModNone),
	))

	viper.SetDefault("keybinding.filter-files", components.NewKeyBinding(
		"Find",
		tcell.NewEventKey(tcell.KeyCtrlF, rune(0), tcell.ModCtrl),
	))
	// keybindings: layer view
	viper.SetDefault("keybinding.compare-all", components.NewKeyBinding(
		"Compare All",
		tcell.NewEventKey(tcell.KeyCtrlA, rune(0), tcell.ModCtrl),
	))

	viper.SetDefault("keybinding.compare-layer", components.NewKeyBinding(
		"Compare Layer",
		tcell.NewEventKey(tcell.KeyCtrlL, rune(0), tcell.ModCtrl),
	))

	// keybindings: filetree view
	viper.SetDefault("keybinding.toggle-collapse-dir", components.NewKeyBinding(
		"Collapse",
		tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone),
	))

	viper.SetDefault("keybinding.toggle-collapse-all-dir", components.NewKeyBinding(
		"Collapse All",
		tcell.NewEventKey(tcell.KeyCtrlSpace, rune(0), tcell.ModCtrl),
	))
	viper.SetDefault("keybinding.toggle-filetree-attributes", components.NewKeyBinding(
		"FileTree Attributes",
		tcell.NewEventKey(tcell.KeyCtrlB, rune(0), tcell.ModCtrl),
	))


	viper.SetDefault("keybinding.toggle-added-files", components.NewKeyBinding(
		"Added Files",
		tcell.NewEventKey(tcell.KeyCtrlA, rune(0), tcell.ModCtrl),
	))


	viper.SetDefault("keybinding.toggle-removed-files", components.NewKeyBinding(
		"Removed Files",
		tcell.NewEventKey(tcell.KeyCtrlR, rune(0), tcell.ModCtrl),
	))

	viper.SetDefault("keybinding.toggle-modified-files", components.NewKeyBinding(
		"Modified Files",
		tcell.NewEventKey(tcell.KeyCtrlM, rune(0), tcell.ModCtrl),
	))

	viper.SetDefault("keybinding.toggle-unmodified-files", components.NewKeyBinding(
		"Unmodified Files",
		tcell.NewEventKey(tcell.KeyCtrlU, rune(0), tcell.ModCtrl),
	))

	viper.SetDefault("keybinding.page-up", components.NewKeyBinding(
		"Page Up",
		tcell.NewEventKey(tcell.KeyPgUp, rune(0), tcell.ModNone),
	))

	viper.SetDefault("keybinding.page-down", components.NewKeyBinding(
		"Page Down",
		tcell.NewEventKey(tcell.KeyPgDn, rune(0), tcell.ModNone),
	))

	viper.SetDefault("diff.hide", "")

	viper.SetDefault("layer.show-aggregated-changes", false)

	viper.SetDefault("filetree.collapse-dir", false)
	viper.SetDefault("filetree.pane-width", 0.5)
	viper.SetDefault("filetree.show-attributes", true)

	viper.SetDefault("container-engine", "docker")
	viper.SetDefault("ignore-errors", false)

	err = viper.BindPFlag("source", rootCmd.PersistentFlags().Lookup("source"))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	viper.SetEnvPrefix("DIVE")
	// replace all - with _ when looking for matching environment variables
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	// if config files are present, load them
	if cfgFile == "" {
		// default configs are ignored if not found
		filepathToCfg := getDefaultCfgFile()
		viper.SetConfigFile(filepathToCfg)
	} else {
		viper.SetConfigFile(cfgFile)
	}
	err = viper.ReadInConfig()
	if err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	} else if cfgFile != "" {
		fmt.Println(err)
		os.Exit(0)
	}

	// set global defaults (for performance)
	filetree.GlobalFileTreeCollapse = viper.GetBool("filetree.collapse-dir")
}

// initLogging sets up the logging object with a formatter and location
func initLogging() {
	var logFileObj *os.File
	var err error

	if viper.GetBool("log.enabled") {
		logFileObj, err = os.OpenFile(viper.GetString("log.path"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		log.SetOutput(logFileObj)
	} else {
		log.SetOutput(ioutil.Discard)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	Formatter := new(log.TextFormatter)
	Formatter.DisableTimestamp = true
	log.SetFormatter(Formatter)

	level, err := log.ParseLevel(viper.GetString("log.level"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	log.SetLevel(level)
	log.Debug("Starting Dive...")
	log.Debugf("config filepath: %s", viper.ConfigFileUsed())
	for k, v := range viper.AllSettings() {
		log.Debug("config value: ", k, " : ", v)
	}
}

// getDefaultCfgFile checks for config file in paths from xdg specs
// and in $HOME/.config/dive/ directory
// defaults to $HOME/.dive.yaml
func getDefaultCfgFile() string {
	home, err := homedir.Dir()
	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}

	xdgHome := os.Getenv("XDG_CONFIG_HOME")
	xdgDirs := os.Getenv("XDG_CONFIG_DIRS")
	xdgPaths := append([]string{xdgHome}, strings.Split(xdgDirs, ":")...)
	allDirs := append(xdgPaths, path.Join(home, ".config"))

	for _, val := range allDirs {
		file := findInPath(val)
		if len(file) > 0 {
			return file
		}
	}
	return path.Join(home, ".dive.yaml")
}

// findInPath returns first "*.yaml" file in path's subdirectory "dive"
// if not found returns empty string
func findInPath(pathTo string) string {
	directory := path.Join(pathTo, "dive")
	files, err := ioutil.ReadDir(directory)
	if err != nil {
		return ""
	}

	for _, file := range files {
		filename := file.Name()
		if path.Ext(filename) == ".yaml" {
			return path.Join(directory, filename)
		}
	}
	return ""
}
