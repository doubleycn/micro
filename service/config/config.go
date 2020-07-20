package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/micro/cli/v2"
	"github.com/micro/go-micro/v2"
	"github.com/micro/go-micro/v2/cmd"
	proto "github.com/micro/go-micro/v2/config/source/service/proto"
	log "github.com/micro/go-micro/v2/logger"
	"github.com/micro/micro/v2/client/cli/namespace"
	"github.com/micro/micro/v2/client/cli/util"
	"github.com/micro/micro/v2/internal/client"
	cliconfig "github.com/micro/micro/v2/internal/config"
	"github.com/micro/micro/v2/internal/helper"
	"github.com/micro/micro/v2/service/config/handler"
)

var (
	// Service name
	Name = "go.micro.config"
	// Default database store
	Database = "store"
	// Flags specific to the config service
	Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "namespace",
			EnvVars: []string{"MICRO_CONFIG_NAMESPACE"},
			Usage:   "Set the namespace used by the Config Service e.g. go.micro.srv.config",
		},
		&cli.StringFlag{
			Name:    "watch_topic",
			EnvVars: []string{"MICRO_CONFIG_WATCH_TOPIC"},
			Usage:   "watch the change event.",
		},
	}
	subcommandFlags = []cli.Flag{
		&cli.BoolFlag{
			Name:  "local",
			Usage: "Connect to local user micro config file and not to micro server config",
		},
	}
)

func Run(c *cli.Context, srvOpts ...micro.Option) {
	if len(c.String("server_name")) > 0 {
		Name = c.String("server_name")
	}

	if len(c.String("watch_topic")) > 0 {
		handler.WatchTopic = c.String("watch_topic")
	}

	srvOpts = append(srvOpts, micro.Name(Name))

	service := micro.NewService(srvOpts...)

	h := &handler.Config{
		Store: *cmd.DefaultCmd.Options().Store,
	}

	proto.RegisterConfigHandler(service.Server(), h)
	micro.RegisterSubscriber(handler.WatchTopic, service.Server(), handler.Watcher)

	if err := service.Run(); err != nil {
		log.Fatalf("config Run the service error: ", err)
	}
}

func setConfig(ctx *cli.Context) error {
	args := ctx.Args()
	// key val
	key := args.Get(0)
	val := args.Get(1)

	if ctx.Bool("local") {
		return cliconfig.Set(val, strings.Split(key, ".")...)
	}
	if len(ctx.String("server_name")) > 0 {
		Name = ctx.String("server_name")
	}
	pb := proto.NewConfigService(Name, client.New(ctx))

	if args.Len() == 0 {
		fmt.Println("Required usage: micro config set key val")
		os.Exit(1)
	}

	ns, err := namespace.Get(util.GetEnv(ctx).Name)
	if err != nil {
		return err
	}

	// TODO: allow the specifying of a config.Key. This will be service name
	// The actuall key-val set is a path e.g micro/accounts/key
	_, err = pb.Update(context.TODO(), &proto.UpdateRequest{
		Change: &proto.Change{
			// the current namespace
			Namespace: ns,
			// actual key for the value
			Path: key,
			// The value
			ChangeSet: &proto.ChangeSet{
				Data:      string(val),
				Format:    "json",
				Source:    "cli",
				Timestamp: time.Now().Unix(),
			},
		},
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return nil
}

func getConfig(ctx *cli.Context) error {
	args := ctx.Args()

	if args.Len() == 0 {
		fmt.Println("Required usage: micro config get key")
		os.Exit(1)
	}
	// key val
	key := args.Get(0)
	if len(key) == 0 {
		log.Fatal("key cannot be blank")
	}

	if ctx.Bool("local") {
		val, err := cliconfig.Get(strings.Split(key, ".")...)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
			return err
		}
		fmt.Println(val)
		return err
	}

	if len(ctx.String("server_name")) > 0 {
		Name = ctx.String("server_name")
	}
	pb := proto.NewConfigService(Name, client.New(ctx))

	ns, err := namespace.Get(util.GetEnv(ctx).Name)
	if err != nil {
		return err
	}

	// TODO: allow the specifying of a config.Key. This will be service name
	// The actuall key-val set is a path e.g micro/accounts/key

	rsp, err := pb.Read(context.TODO(), &proto.ReadRequest{
		// The current namespace,
		Namespace: ns,
		// The actual key for the val
		Path: key,
	})

	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			fmt.Println("not found")
			os.Exit(1)
		}
		fmt.Println(err)
		os.Exit(1)
	}

	if rsp.Change == nil || rsp.Change.ChangeSet == nil {
		fmt.Println("not found")
		os.Exit(1)
	}

	// don't do it
	if v := rsp.Change.ChangeSet.Data; len(v) == 0 || string(v) == "null" {
		fmt.Println("not found")
		os.Exit(1)
	}

	fmt.Println(string(rsp.Change.ChangeSet.Data))

	return nil
}

func delConfig(ctx *cli.Context) error {
	args := ctx.Args()

	if args.Len() == 0 {
		fmt.Println("Required usage: micro config get key")
		os.Exit(1)
	}
	// key val
	key := args.Get(0)
	if len(key) == 0 {
		log.Fatal("key cannot be blank")
	}

	if len(ctx.String("server_name")) > 0 {
		Name = ctx.String("server_name")
	}
	pb := proto.NewConfigService(Name, client.New(ctx))

	ns, err := namespace.Get(util.GetEnv(ctx).Name)
	if err != nil {
		return err
	}

	// TODO: allow the specifying of a config.Key. This will be service name
	// The actuall key-val set is a path e.g micro/accounts/key

	_, err = pb.Delete(context.TODO(), &proto.DeleteRequest{
		Change: &proto.Change{
			// The current namespace
			Namespace: ns,
			// The actual key for the val
			Path: key,
		},
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return nil
}

func Commands(options ...micro.Option) []*cli.Command {
	command := &cli.Command{
		Name:   "config",
		Usage:  "Manage configuration values",
		Action: helper.UnexpectedSubcommand,
		Subcommands: []*cli.Command{
			{
				Name:   "get",
				Usage:  "Get a value; micro config get key",
				Action: getConfig,
				Flags:  subcommandFlags,
			},
			{
				Name:   "set",
				Usage:  "Set a key-val; micro config set key val",
				Action: setConfig,
				Flags:  subcommandFlags,
			},
			{
				Name:   "del",
				Usage:  "Delete a value; micro config del key",
				Action: delConfig,
				Flags:  subcommandFlags,
			},
		},
	}

	return []*cli.Command{command}
}
