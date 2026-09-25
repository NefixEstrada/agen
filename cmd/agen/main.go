// Command agen generates typed, validating Go code from AsyncAPI 3.x
// documents.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-faster/errors"
	"github.com/go-faster/yaml"
	"go.uber.org/zap"

	"github.com/ogen-go/ogen/gen/genfs"
	"github.com/ogen-go/ogen/jsonschema"
	"github.com/ogen-go/ogen/location"

	"github.com/NefixEstrada/agen/asyncapi"
	parser "github.com/NefixEstrada/agen/asyncapi/parser"
	"github.com/NefixEstrada/agen/gen"
	agenconfig "github.com/NefixEstrada/agen/internal/config"
)

func main() {
	if err := run(); err != nil {
		if !location.PrintPrettyError(os.Stderr, true, err) {
			_, _ = fmt.Fprintf(os.Stderr, "%+v\n", err)
		}
		os.Exit(1)
	}
}

func run() error {
	set := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	set.Usage = func() {
		_, toolName := filepath.Split(os.Args[0])
		_, _ = fmt.Fprintf(set.Output(), "%s generates Go code from AsyncAPI 3.x documents\n\n", toolName)
		_, _ = fmt.Fprintf(set.Output(), "Usage:\n  %s [flags] [spec file] [-- package dir]\n\n", toolName)
		set.PrintDefaults()
	}

	cfgPath := set.String("config", "", "Path to config file (auto-discovered: agen.yml, .agen.yml, ...)")
	verbose := set.Bool("v", false, "Enable verbose mode")
	_ = set.Parse(os.Args[1:])

	log := zap.NewNop()
	if *verbose {
		l, err := zap.NewDevelopment()
		if err != nil {
			return errors.Wrap(err, "logger")
		}
		log = l
	}

	if *cfgPath == "" {
		if p, ok := agenconfig.AutoDiscover("."); ok {
			*cfgPath = p
			log.Debug("Using config file", zap.String("path", p))
		}
	}

	cfg := &agenconfig.Config{}
	if *cfgPath != "" {
		c, err := agenconfig.Load(*cfgPath)
		if err != nil {
			return errors.Wrap(err, "load config")
		}
		cfg = c
	}

	// Positional arguments override the config file: spec and target dir.
	args := set.Args()
	switch len(args) {
	case 0:
		if cfg.Target.Spec == "" {
			set.Usage()
			return errors.New("no spec provided")
		}
	case 1:
		cfg.Target.Spec = args[0]
	case 2:
		cfg.Target.Spec = args[0]
		cfg.Target.Dir = args[1]
	default:
		set.Usage()
		return errors.New("too many arguments")
	}

	if err := Generate(cfg, log); err != nil {
		return err
	}
	return nil
}

// Generate runs the full pipeline: load → parse → generate → write.
func Generate(cfg *agenconfig.Config, log *zap.Logger) error {
	specPath := filepath.Clean(cfg.Target.Spec)

	data, rootURL, err := loadSpec(specPath)
	if err != nil {
		return errors.Wrap(err, "load spec")
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return errors.Wrap(err, "parse yaml")
	}

	// Rewrite x-agen-* extensions to their x-ogen-* equivalents so the
	// vendored ogen engine honors them.
	asyncapi.AliasXExtensions(&root)

	var spec asyncapi.Spec
	if err := root.Decode(&spec); err != nil {
		return errors.Wrap(err, "decode spec")
	}
	if err := checkVersionEarly(spec.AsyncAPI); err != nil {
		return err
	}

	var external jsonschema.ExternalResolver
	if cfg.Parser.AllowRemote {
		external = jsonschema.NewExternalResolver(jsonschema.ExternalOptions{
			Logger: log,
		})
	}

	specAbs := specPath
	if abs, err := filepath.Abs(specPath); err == nil {
		specAbs = abs
	}
	api, err := parser.Parse(&spec, &root, rootURL, parser.Config{
		External:   external,
		InferTypes: cfg.Parser.InferTypes,
		DepthLimit: cfg.Parser.DepthLimit,
		Logger:     log,
		RootFile:   location.NewFile(specAbs, specAbs, data),
	})
	if err != nil {
		return err
	}

	features, err := gen.BuildFeatures(
		featureList(cfg.Generator.Features.Enable),
		featureList(cfg.Generator.Features.Disable),
		cfg.Generator.Features.DisableAll,
	)
	if err != nil {
		return errors.Wrap(err, "generator.features")
	}

	var (
		opsRegex *regexp.Regexp
	)
	if r := cfg.Generator.Filters.OperationsRegex; r != "" {
		opsRegex, err = regexp.Compile(r)
		if err != nil {
			return errors.Wrap(err, "generator.filters.operations_regex")
		}
	}

	g, err := gen.NewGenerator(api, gen.Options{
		Features:          features,
		Filters:           gen.Filters{OperationsRegex: opsRegex, Actions: cfg.Generator.Filters.Actions},
		IgnoreUnsupported: cfg.Parser.IgnoreUnsupported,
		Initialisms:       cfg.Generator.Initialisms,
		Logger:            log,
	})
	if err != nil {
		return err
	}

	pkgName := cfg.PackageName(api.Info.Title)
	if err := os.MkdirAll(cfg.Target.Dir, 0o750); err != nil {
		return errors.Wrap(err, "create target dir")
	}
	fs := genfs.FormattedSource{
		Root:   cfg.Target.Dir,
		Format: true,
	}
	if err := g.WriteSource(fs, pkgName); err != nil {
		return err
	}

	if out := cfg.Expand.Output; out != "" {
		if err := asyncapi.Expand(&root); err != nil {
			return errors.Wrap(err, "expand")
		}
		data, err := yaml.Marshal(root.Content[0])
		if err != nil {
			return errors.Wrap(err, "marshal expanded spec")
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			return errors.Wrap(err, "write expanded spec")
		}
		fmt.Printf("Expanded spec written to %s\n", out)
	}

	fmt.Printf("Generated package %q in %s\n", pkgName, cfg.Target.Dir)
	return nil
}

func featureList(fs []string) []gen.Feature {
	if len(fs) == 0 {
		return nil
	}
	out := make([]gen.Feature, 0, len(fs))
	for _, f := range fs {
		out = append(out, gen.Feature(f))
	}
	return out
}

func checkVersionEarly(version string) error {
	if version == "" {
		return errors.New(`invalid AsyncAPI document: the "asyncapi" field is required`)
	}
	major, _, _ := strings.Cut(version, ".")
	if major == "2" || major == "1" {
		return errors.Errorf(
			"AsyncAPI %s is not supported: agen only reads 3.x documents. "+
				"Upgrade the document first, e.g. with the official @asyncapi/converter.", version)
	}
	return nil
}

func loadSpec(path string) ([]byte, *url.URL, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		u, err := url.Parse(path)
		if err != nil {
			return nil, nil, errors.Wrap(err, "parse url")
		}
		//#nosec G107
		resp, err := httpGet(u.String())
		if err != nil {
			return nil, nil, errors.Wrap(err, "fetch spec")
		}
		return resp, u, nil
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, errors.Wrap(err, "abs path")
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, nil, errors.Wrap(err, "read file")
	}
	u, err := url.Parse("file://" + abs)
	if err != nil {
		return nil, nil, errors.Wrap(err, "url from path")
	}
	return data, u, nil
}

func httpGet(url string) ([]byte, error) {
	//#nosec G107
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("unexpected status %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}
