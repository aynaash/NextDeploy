
	bootstrapCmd := flag.NewFlagSet("bootstrap", flag.ExitOnError)
	bootstrapEmail := bootstrapCmd.String("email", "", "Your email address")
	bootstrapServer := bootstrapCmd.String("server", "", "Daemon server address")

	allowCmd := flag.NewFlagSet("allow", flag.ExitOnError)
	allowUser := allowCmd.String("user", "", "User email to allow")
	allowRole := allowCmd.String("role", shared.RoleDeployer, "Role to assign")
	allowServer := allowCmd.String("server", "", "Daemon server address")

	revokeCmd := flag.NewFlagSet("revoke", flag.ExitOnError)
	revokeUser := revokeCmd.String("user", "", "User email to revoke")
	revokeServer := revokeCmd.String("server", "", "Daemon server address")

	if len(os.Args) < 2 {
		fmt.Println("expected 'bootstrap', 'allow', or 'revoke' subcommands")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "bootstrap":
		bootstrapCmd.Parse(os.Args[2:])
		if *bootstrapEmail == "" || *bootstrapServer == "" {
			bootstrapCmd.Usage()
			os.Exit(1)
		}

		cli, err := NewCLI()
		if err != nil {
			log.Fatal(err)
		}

		if err := cli.BootstrapTrust(*bootstrapServer, *bootstrapEmail); err != nil {
			log.Fatal(err)
		}

		fmt.Println("Successfully bootstrapped trust with daemon")

	case "allow":
		allowCmd.Parse(os.Args[2:])
		if *allowUser == "" || *allowServer == "" {
			allowCmd.Usage()
			os.Exit(1)
		}

		cli, err := NewCLI()
		if err != nil {
			log.Fatal(err)
		}

		if err := cli.AddIdentity(*allowServer, *allowUser, *allowRole); err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Successfully added %s as %s\n", *allowUser, *allowRole)

	case "revoke":
		revokeCmd.Parse(os.Args[2:])
		if *revokeUser == "" || *revokeServer == "" {
			revokeCmd.Usage()
			os.Exit(1)
		}

		cli, err := NewCLI()
		if err != nil {
			log.Fatal(err)
		}

		if err := cli.RevokeIdentity(*revokeServer, *revokeUser); err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Successfully revoked access for %s\n", *revokeUser)

	default:
		fmt.Println("expected 'bootstrap', 'allow', or 'revoke' subcommands")
		os.Exit(1)
	}
