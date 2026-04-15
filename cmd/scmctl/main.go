package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/user"
	"time"

	manifestapp "github.com/alexisjcarr/scm/internal/manifest/app"
	"github.com/alexisjcarr/scm/internal/platform/config"
	"github.com/alexisjcarr/scm/internal/platform/grpcutil"
	"github.com/alexisjcarr/scm/internal/version"
	scmv1 "github.com/alexisjcarr/scm/pkg/api/scm/v1"
	"google.golang.org/grpc"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "validate":
		if err := runValidate(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "apply":
		if err := runApply(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "version":
		fmt.Printf("scmctl %s (%s) built %s\n", version.Version, version.Commit, version.BuildDate)
	default:
		usage()
		os.Exit(2)
	}
}

func runValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	manifestPath := fs.String("f", "", "path to manifest YAML")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *manifestPath == "" {
		return fmt.Errorf("-f is required")
	}

	compiled, _, err := manifestapp.Service{}.LoadFile(*manifestPath)
	if err != nil {
		return err
	}
	fmt.Printf("manifest %q is valid (%d resources, %d targets declared)\n", compiled.Name, len(compiled.Resources), len(compiled.Target.Hosts))
	return nil
}

func runApply(args []string) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	manifestPath := fs.String("f", "", "path to manifest YAML")
	configPath := fs.String("config", "~/.config/scm/scmctl.yaml", "path to scmctl config")
	serverAddress := fs.String("server", "", "override control plane address")
	watch := fs.Bool("watch", false, "stream apply output until completion")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *manifestPath == "" {
		return fmt.Errorf("-f is required")
	}

	cfg, err := config.LoadCLIConfig(*configPath)
	if err != nil {
		return err
	}
	if *serverAddress != "" {
		cfg.ServerAddress = *serverAddress
	}

	compiled, raw, err := manifestapp.Service{}.LoadFile(*manifestPath)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, cfg.ServerAddress, grpcutil.DialOptions()...)
	if err != nil {
		return fmt.Errorf("dial control plane: %w", err)
	}
	defer conn.Close()

	client := scmv1.NewApplyServiceClient(conn)
	response, err := client.SubmitApply(ctx, &scmv1.SubmitApplyRequest{
		Manifest:    compiled.ToAPI(),
		RawManifest: string(raw),
		SubmittedBy: currentUser(),
	})
	if err != nil {
		return fmt.Errorf("submit apply: %w", err)
	}

	fmt.Printf("apply_id=%s status=%s targets=%d\n", response.ApplyID, response.Status, response.TargetCount)
	if !*watch {
		return nil
	}

	streamCtx, streamCancel := context.WithCancel(context.Background())
	defer streamCancel()
	stream, err := client.StreamApplyEvents(streamCtx, &scmv1.StreamApplyEventsRequest{ApplyID: response.ApplyID})
	if err != nil {
		return fmt.Errorf("stream apply events: %w", err)
	}
	for {
		event, err := stream.Recv()
		if err != nil {
			break
		}
		fmt.Printf("%s %-10s %-10s %s\n", event.CreatedAt, event.HostID, event.Phase, event.Message)
	}

	statusCtx, statusCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer statusCancel()
	apply, err := client.GetApply(statusCtx, &scmv1.GetApplyRequest{ApplyID: response.ApplyID})
	if err != nil {
		return fmt.Errorf("get apply status: %w", err)
	}
	fmt.Printf("final status: %s\n", apply.Status)
	return nil
}

func currentUser() string {
	u, err := user.Current()
	if err != nil {
		return "unknown"
	}
	return u.Username
}

func usage() {
	fmt.Println("usage: scmctl <validate|apply|version> [flags]")
}
