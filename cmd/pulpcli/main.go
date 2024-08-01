package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/clients/pulp"
	"github.com/sirupsen/logrus"
)

func domainList(ctx context.Context, c *pulp.PulpService) {
	domains, err := c.DomainsList(ctx, "")
	if err != nil {
		panic(err)
	}

	for _, d := range domains {
		fmt.Println(d.Name, *d.PulpHref)
	}
}

// nolint: gosec
var rnd = rand.New(rand.NewSource(time.Now().UnixNano()))

func fixtureCreate(ctx context.Context, c *pulp.PulpService, orgID, _ string) {
	resourceName := fmt.Sprintf("test-%d", rnd.Int())

	frepo, err := c.FileRepositoriesEnsure(ctx)
	if err != nil {
		panic(err)
	}
	fmt.Println("File repo found or created", frepo)
	fmt.Println("--------------------------------")

	artifact, version, err := c.FileRepositoriesImport(ctx, frepo, "https://home.zapletalovi.com/small_commit1.tar")
	if err != nil {
		panic(err)
	}

	fmt.Println("Artifact uploaded", artifact)
	fmt.Println("--------------------------------")

	repo, err := c.RepositoriesCreate(ctx, resourceName)
	if err != nil {
		panic(err)
	}
	fmt.Println("Repository created", *repo.PulpHref)
	fmt.Println("--------------------------------")

	cg, err := c.ContentGuardEnsure(ctx, orgID)
	if err != nil {
		panic(err)
	}
	fmt.Println("Content guard found or created", *cg.PulpHref, (*cg.Guards)[0], (*cg.Guards)[1])
	fmt.Println("--------------------------------")

	dist, err := c.DistributionsCreate(ctx, resourceName, resourceName, *repo.PulpHref, *cg.PulpHref)
	if err != nil {
		panic(err)
	}
	fmt.Println("Distribution created", *dist.PulpHref)
	fmt.Println("--------------------------------")

	repoImported, err := c.RepositoriesImport(ctx, pulp.ScanUUID(repo.PulpHref), "repo", artifact)
	if err != nil {
		panic(err)
	}
	fmt.Println("Repository imported", *repoImported.PulpHref)
	fmt.Println("--------------------------------")

	err = c.FileRepositoriesVersionDelete(ctx, pulp.ScanUUID(&version), pulp.ScanRepoFileVersion(&version))
	if err != nil {
		panic(err)
	}
	fmt.Println("Artifact version deleted", version)
	fmt.Println("--------------------------------")

	fmt.Printf("curl -L --proxy http://squid.xxxx.redhat.com:3128 --cert /etc/pki/consumer/cert.pem --key /etc/pki/consumer/key.pem https://cert.console.stage.redhat.com/api/pulp-content/%s/%s/\n", c.Domain(), resourceName)
	fmt.Printf("curl -L --proxy http://squid.xxxx.redhat.com:3128 -u edge-content-dev:XXXX https://pulp.stage.xxxx.net/api/pulp-content/%s/%s/\n", c.Domain(), resourceName)
	fmt.Println("--------------------------------")
}

func main() {
	ctx := context.Background()
	config.Init()
	logger.InitLogger(os.Stdout)
	logrus.SetLevel(logrus.TraceLevel)

	// when changing the domain, please delete all artifacts, repos, distros via CLI and then delete the domain
	domainName := "edge-integration-test-2"
	domainHref := uuid.MustParse("0190360e-c33e-73d3-a666-591cd2730da9")

	c, err := pulp.NewPulpServiceDefaultDomain(ctx)
	if err != nil {
		panic(err)
	}

	_, err = c.DomainsRead(ctx, domainName, domainHref)
	if err != nil {
		createdDomain, err := c.DomainsCreate(ctx, domainName)
		if err != nil {
			panic(err)
		}
		fmt.Println("Created domain:", pulp.ScanUUID(createdDomain.PulpHref), ", please update the domainHref in the test source!")
		return
	}

	c, err = pulp.NewPulpServiceWithDomain(ctx, domainName)
	if err != nil {
		panic(err)
	}

	// nolint: gocritic
	if len(os.Args) > 1 && os.Args[1] == "domain_list" {
		domainList(ctx, c)
	} else if len(os.Args) > 3 && os.Args[1] == "fixture_create" {
		fixtureCreate(ctx, c, os.Args[2], os.Args[3])
	} else {
		fmt.Println("Usage:")
		fmt.Println("cli domain_list: list all domains")
		fmt.Println("cli fixture_create org_id tar_file: create a fixture test repo")
	}
}
