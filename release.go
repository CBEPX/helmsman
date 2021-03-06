package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	version "github.com/hashicorp/go-version"
)

// release type representing Helm releases which are described in the desired state
type release struct {
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description"`
	Namespace       string   `yaml:"namespace"`
	Enabled         bool     `yaml:"enabled"`
	Chart           string   `yaml:"chart"`
	Version         string   `yaml:"version"`
	ValuesFile      string   `yaml:"valuesFile"`
	ValuesFiles     []string `yaml:"valuesFiles"`
	SecretFile      string   `yaml:"secretFile"`
	SecretFiles     []string `yaml:"secretFiles"`
	Purge           bool     `yaml:"purge"`
	Test            bool     `yaml:"test"`
	Protected       bool     `yaml:"protected"`
	Wait            bool     `yaml:"wait"`
	Priority        int      `yaml:"priority"`
	TillerNamespace string   `yaml:"tillerNamespace"`
	Set             map[string]string
	SetString       map[string]string `yaml:"setString"`
	NoHooks         bool              `yaml:"noHooks"`
	Timeout         int
}

// validateRelease validates if a release inside a desired state meets the specifications or not.
// check the full specification @ https://github.com/Praqma/helmsman/docs/desired_state_spec.md
func validateRelease(appLabel string, r *release, names map[string]map[string]bool, s state) (bool, string) {
	_, err := os.Stat(pwd + "/" + relativeDir + "/" + r.ValuesFile)
	if r.Name == "" {
		r.Name = appLabel
	}
	if r.TillerNamespace != "" {
		if ns, ok := s.Namespaces[r.TillerNamespace]; !ok {
			return false, "tillerNamespace specified, but the namespace specified does not exist!"
		} else if !ns.InstallTiller && !ns.UseTiller {
			return false, "tillerNamespace specified, but that namespace does not have neither installTiller nor useTiller set to true."
		}
	} else if getDesiredTillerNamespace(r) == "kube-system" {
		if ns, ok := s.Namespaces["kube-system"]; ok && !ns.InstallTiller && !ns.UseTiller {
			return false, "app is desired to be deployed using Tiller from [[ kube-system ]] but kube-system is not desired to have a Tiller installed nor use an existing Tiller. You can use another Tiller with the 'tillerNamespace' option or deploy Tiller in kube-system. "
		}
	}
	if names[r.Name][getDesiredTillerNamespace(r)] {
		return false, "release name must be unique within a given Tiller."
	}

	if nsOverride == "" && r.Namespace == "" {
		return false, "release targeted namespace can't be empty."
	} else if nsOverride == "" && r.Namespace != "" && r.Namespace != "kube-system" && !checkNamespaceDefined(r.Namespace, s) {
		return false, "release " + r.Name + " is using namespace [ " + r.Namespace + " ] which is not defined in the Namespaces section of your desired state file." +
			" Release [ " + r.Name + " ] can't be installed in that Namespace until its defined."
	}
	if r.Chart == "" || !strings.ContainsAny(r.Chart, "/") {
		return false, "chart can't be empty and must be of the format: repo/chart."
	}
	if r.Version == "" {
		return false, "version can't be empty."
	}
	r.Version = substituteEnv(r.Version)

	if r.ValuesFile != "" && (!isOfType(r.ValuesFile, ".yaml") || err != nil) {
		return false, "valuesFile must be a valid relative (from your first dsf file) file path for a yaml file, Or can be left empty."
	} else if r.ValuesFile != "" && len(r.ValuesFiles) > 0 {
		return false, "valuesFile and valuesFiles should not be used together."
	} else if len(r.ValuesFiles) > 0 {
		for _, filePath := range r.ValuesFiles {
			if _, pathErr := os.Stat(pwd + "/" + relativeDir + "/" + filePath); !isOfType(filePath, ".yaml") || pathErr != nil {
				return false, "the value for valueFile '" + filePath + "' must be a valid relative (from your first dsf file) file path for a yaml file."
			}
		}
	}

	if r.SecretFile != "" && (!isOfType(r.SecretFile, ".yaml") || err != nil) {
		return false, "secretFile must be a valid relative (from your first dsf file) file path for a yaml file, Or can be left empty."
	} else if r.SecretFile != "" && len(r.SecretFiles) > 0 {
		return false, "secretFile and secretFiles should not be used together."
	} else if len(r.SecretFiles) > 0 {
		for _, filePath := range r.SecretFiles {
			if _, pathErr := os.Stat(pwd + "/" + relativeDir + "/" + filePath); !isOfType(filePath, ".yaml") || pathErr != nil {
				return false, "the value for valueFile '" + filePath + "' must be a valid relative (from your first dsf file) file path for a yaml file."
			}
		}
	}

	if r.Priority != 0 && r.Priority > 0 {
		return false, "priority can only be 0 or negative value, positive values are not allowed."
	}

	if names[r.Name] == nil {
		names[r.Name] = make(map[string]bool)
	}
	if r.TillerNamespace != "" {
		names[r.Name][r.TillerNamespace] = true
	} else if s.Namespaces[r.Namespace].InstallTiller {
		names[r.Name][r.Namespace] = true
	} else {
		names[r.Name]["kube-system"] = true
	}

	if len(r.SetString) > 0 {
		v1, _ := version.NewVersion(helmVersion)
		setStringConstraint, _ := version.NewConstraint(">=2.9.0")
		if !setStringConstraint.Check(v1) {
			return false, "you are using setString in your desired state, but your helm client does not support it. You need helm v2.9.0 or above for this feature."
		}
	}

	return true, ""
}

// checkNamespaceDefined checks if a given namespace is defined in the namespaces section of the desired state file
func checkNamespaceDefined(ns string, s state) bool {
	_, ok := s.Namespaces[ns]
	if !ok {
		return false
	}
	return true
}

// overrideNamespace overrides a release defined namespace with a new given one
func overrideNamespace(r *release, newNs string) {
	log.Println("INFO: overriding namespace for app:  " + r.Name)
	r.Namespace = newNs
}

// print prints the details of the release
func (r release) print() {
	fmt.Println("")
	fmt.Println("\tname : ", r.Name)
	fmt.Println("\tdescription : ", r.Description)
	fmt.Println("\tnamespace : ", r.Namespace)
	fmt.Println("\tenabled : ", r.Enabled)
	fmt.Println("\tchart : ", r.Chart)
	fmt.Println("\tversion : ", r.Version)
	fmt.Println("\tvaluesFile : ", r.ValuesFile)
	fmt.Println("\tvaluesFiles : ", strings.Join(r.ValuesFiles, ","))
	fmt.Println("\tpurge : ", r.Purge)
	fmt.Println("\ttest : ", r.Test)
	fmt.Println("\tprotected : ", r.Protected)
	fmt.Println("\twait : ", r.Wait)
	fmt.Println("\tpriority : ", r.Priority)
	fmt.Println("\ttiller namespace : ", r.TillerNamespace)
	fmt.Println("\tno-hooks : ", r.NoHooks)
	fmt.Println("\ttimeout : ", r.Timeout)
	fmt.Println("\tvalues to override from env:")
	printMap(r.Set)
	fmt.Println("------------------- ")
}
