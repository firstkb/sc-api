package config

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/knadh/koanf/parsers/hjson"
	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/parameterstore/v2"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
)

/*
Precedence of settings (highest to lowest):
  - A config source thats name is declared on the command line
  - Environment variables
  - Local config file (if exists)
    linux  : $HOME/.config/<appname>/.conf
    windows: %USERPROFILE%/<appname>/.conf
  - Global config file (if exists)
    linux  : /etc/<appname>/.conf
    windows: <application_path>/.conf
  - Hardcoded default values
*/
type Config struct {
	k *koanf.Koanf

	Sources *[]string
}

const (
	defaultConfigFileExt = ".conf"
)

// 'defaultConfiguration' is expected to be in TOML format (see https://toml.io/en)
//
// The key name for the env var will be created using the following rules:
//   - For the first prefix only. The prefix will be stripped from the name
//   - `_` will be replaced with `.`
//   - the name will be converted to lowercase
// Examples:
//  prefixes: "SMARTAPI_", "AWS_"
//  env name: "SMARTAPI_TLS_CERT", "AWS_S3_ACCESS_KEY"
//  key: 	  tls.cert, aws.s3.access.key

func GetConfig(configOptions *configOptions, defaultConfiguration []byte, envPrefixes ...string) (*Config, error) {
	k := koanf.New(".")
	sources := []string{}

	if len(defaultConfiguration) > 0 {
		err := k.Load(rawbytes.Provider(defaultConfiguration), toml.Parser())
		if err != nil {
			//TODO: user friendly parsing error
			return nil, fmt.Errorf("default values: %v", err)
		}
	}

	path, err := getGlobalConfigFilePath()
	if err != nil {
		return nil, fmt.Errorf("global config: %v", err)
	} else {
		err = loadFile(k, path)
		if err == nil {
			sources = append(sources, path)
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("%s: %v", path, err)
		}
	}

	path, err = getLocalConfigFilePath()
	if err != nil {
		return nil, fmt.Errorf("local config: %v", err)
	} else {
		err = loadFile(k, path)
		if err == nil {
			sources = append(sources, path)
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("%s: %v", path, err)
		}
	}

	for index, prefix := range envPrefixes {
		if len(prefix) > 0 {
			err = k.Load(env.Provider(prefix, ".", func(s string) string {
				if index == 0 {
					s = strings.TrimPrefix(s, prefix)
				}
				return strings.ReplaceAll(strings.ToLower(s), "_", ".")
			}), nil)
			if err != nil {
				return nil, fmt.Errorf("environment variables: %v", err)
			}
		}
	}

	/*
		[file://]<filepath>.conf|.toml|.json|.yaml|
		ssm://<path>
		ssm+conf://<name>
		ssm+toml://<name>
		ssm+json://<name>
		ssm+yaml://<name>
		sm://<name>
		sm+conf://<name>
		sm+toml://<name>
		sm+json://<name>
		sm+yaml://<name>
	*/
	if configOptions.Source != "" {
		u, err := url.Parse(strings.ReplaceAll(configOptions.Source, "\\", "/"))
		if err != nil {
			return nil, err
		}

		source := strings.ToLower(u.Scheme)

		if source == "" || source == "file" {
			path, err = filepath.Abs(filepath.Join(u.Host, u.Path))
			if err == nil {
				err = loadFile(k, path)
			}
		} else if source == "ssm" || source == "ssm+conf" || source == "ssm+toml" || source == "ssm+json" || source == "ssm+yaml" {
			path = u.Host + u.Path
			pr := ""
			if sa := strings.Split(source, "+"); len(sa) > 1 {
				pr = sa[1]
			}
			err = loadParameterStore(k, path, pr)
		} else if source == "sm" || source == "sm+json" || source == "sm+toml" || source == "sm+yaml" {
			path = u.Host + u.Path
			pr := ""
			if sa := strings.Split(source, "+"); len(sa) > 1 {
				pr = sa[1]
			}
			err = loadSecretManager(k, path, pr)
		} else {
			err = fmt.Errorf("invalid or unsupported configuration source '%s'", configOptions.Source)
		}

		if err != nil {
			return nil, err
		}

		sources = append(sources, path)
	}

	return &Config{
		k:       k,
		Sources: &sources,
	}, nil
}

func (cfg *Config) Bool(path string) bool {
	return cfg.k.Bool(path)
}

func (cfg *Config) Bools(path string) []bool {
	return cfg.k.Bools(path)
}

func (cfg *Config) Int(path string) int {
	return cfg.k.Int(path)
}

func (cfg *Config) Ints(path string) []int {
	return cfg.k.Ints(path)
}

func (cfg *Config) String(path string) string {
	return cfg.k.String(path)
}

func (cfg *Config) Strings(path string) []string {
	return cfg.k.Strings(path)
}

func (cfg *Config) Unmarshal(path string, o any) error {
	return cfg.k.UnmarshalWithConf(path, o, koanf.UnmarshalConf{Tag: "json"})
}

func (cfg *Config) Set(path string, value any) error {
	return cfg.k.Set(path, value)
}

func (cfg *Config) Keys() []string {
	return cfg.k.Keys()
}

func getParser(name string) (koanf.Parser, error) {
	switch name {
	case "toml", "conf":
		return toml.Parser(), nil
	case "json":
		return hjson.Parser(), nil
	case "yaml":
		return yaml.Parser(), nil
	}

	return nil, errors.New("unsupported configuration format")
}

func loadFile(k *koanf.Koanf, path string) error {
	//var err error = nil
	//var parser koanf.Parser = nil

	parser, err := getParser(strings.ToLower(strings.Replace(filepath.Ext(path), ".", "", 1)))
	if err != nil {
		return err
	}

	err = k.Load(file.Provider(path), parser)
	if err != nil {
		//TODO: user friendly parsing error
		return err
	}

	return nil
}

func loadParameterStore(k *koanf.Koanf, path string, parserName string) error {
	var parser koanf.Parser = nil

	if parserName != "" {
		pr, err := getParser(parserName)
		if err != nil {
			return err
		}
		parser = pr
	}

	c, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	client := ssm.NewFromConfig(c)

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	if parser == nil {
		if err := k.Load(parameterstore.ProviderWithClient(parameterstore.Config[ssm.GetParametersByPathInput]{
			Delim: ".",
			Input: ssm.GetParametersByPathInput{Path: aws.String(path), WithDecryption: aws.Bool(true)},
		}, client), nil); err != nil {
			//TODO: user friendly parsing error
			return err
		}
	} else {
		result, err := client.GetParameter(context.TODO(), &ssm.GetParameterInput{Name: aws.String(path), WithDecryption: aws.Bool(true)})
		if err != nil {
			//TODO: user friendly error
			return err
		}

		err = k.Load(rawbytes.Provider([]byte(*result.Parameter.Value)), parser)
		if err != nil {
			//TODO: user friendly parsing error
			return err
		}
	}

	//TODO: NOT TESTED
	return errors.New("parameterStore is not implemented yet")
	//return nil
}

func loadSecretManager(k *koanf.Koanf, secretName string, parserName string) error {
	var parser koanf.Parser

	if parserName != "" {
		pr, err := getParser(parserName)
		if err != nil {
			return fmt.Errorf("failed to get parser: %v", err)
		}
		parser = pr
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to load AWS configuration: %v", err)
	}

	client := secretsmanager.NewFromConfig(cfg)

	result, err := client.GetSecretValue(context.TODO(), &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		return fmt.Errorf("failed to retrieve secret: %v", err)
	}

	secretString := *result.SecretString

	if parser == nil {
		if err := k.Load(rawbytes.Provider([]byte(secretString)), nil); err != nil {
			return fmt.Errorf("failed to load secret data without parser: %v", err)
		}
	}

	err = k.Load(rawbytes.Provider([]byte(secretString)), parser)
	if err != nil {
		return fmt.Errorf("failed to load secret data with parser: %v", err)
	}

	return nil
}
