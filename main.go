package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/markbates/pkger"
	"github.com/pkg/errors"

	"github.com/aws/aws-sdk-go-v2/config"

	log "github.com/sirupsen/logrus"

	ds "github.com/goldfiglabs/rpcheckup/pkg/dockersession"
	"github.com/goldfiglabs/rpcheckup/pkg/introspector"
	ps "github.com/goldfiglabs/rpcheckup/pkg/postgres"
	"github.com/goldfiglabs/rpcheckup/pkg/report"
)

func loadAwsCredentials(ctx context.Context) ([]string, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, err
	}
	env := []string{
		fmt.Sprintf("AWS_ACCESS_KEY_ID=%v", creds.AccessKeyID),
		fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%v", creds.SecretAccessKey),
	}
	if len(creds.SessionToken) > 0 {
		env = append(env, fmt.Sprintf("AWS_SESSION_TOKEN=%v", creds.SessionToken))
	}
	return env, nil
}

func printReportRows(rows []report.Row) {
	for _, r := range rows {
		fmt.Printf("Arn %v Service %v Resource %v Is Public %v External Accounts [%v] In-Org Accounts [%v]\n",
			r.Arn, r.Service, r.ProviderType, r.IsPublic, strings.Join(r.ExternalAccounts, ","),
			strings.Join(r.InOrgAccounts, ", "))
	}
}

type templateData struct {
	Report *report.Report
}

var accessColors = map[string]string{
	"Public":            "red",
	"External Accounts": "orange",
	"In-Org Accounts":   "yellow",
	"Private":           "green",
}

func writeCSVReport(rpReport *report.Report, outputFilename string) error {
	outputFile, err := os.Create(outputFilename)
	if err != nil {
		return errors.Wrapf(err, "Failed to create output file %v", outputFilename)
	}
	defer outputFile.Close()
	writer := csv.NewWriter(outputFile)
	defer writer.Flush()
	writer.Write([]string{"ARN", "Service", "Resource", "Access Allows", "In-Org Accounts", "External Accounts", "Is Public"})
	for _, row := range rpReport.Rows {
		writer.Write([]string{
			row.Arn,
			row.Service,
			row.ProviderType,
			row.Access(),
			strings.Join(row.InOrgAccounts, ", "),
			strings.Join(row.ExternalAccounts, ", "),
			strconv.FormatBool(row.IsPublic),
		})
	}
	return nil
}

func writeHTMLReport(rpReport *report.Report, outputFilename string) error {
	filename := "/templates/resource_policies.gohtml"
	f, err := pkger.Open(filename)
	if err != nil {
		return errors.Wrap(err, "Failed to load html template")
	}
	defer f.Close()
	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return errors.Wrap(err, "Failed to read html template")
	}
	t := template.New("rpCheckup")
	t.Funcs(template.FuncMap{
		"yn": func(b bool) string {
			if b {
				return "yes"
			}
			return "no"
		},
		"inc": func(i int) int {
			return i + 1
		},
		"color": func(r *report.Row) string {
			return accessColors[r.Access()]
		},
		"list": func(l []string) string {
			if l == nil || len(l) == 0 {
				return "<NONE>"
			}
			if len(l) > 8 {
				return strings.Join(l[:8], ", ") + "...(+" + strconv.Itoa(len(l)-8) + ")"
			}
			return strings.Join(l, ", ") + " (" + strconv.Itoa(len(l)) + ")"
		},
		"humanize": func(t time.Time) string {
			return t.Format(time.RFC1123)
		},
	})
	t, err = t.Parse(string(bytes))
	if err != nil {
		return errors.Wrap(err, "Failed to parse template")
	}
	outputFile, err := os.Create(outputFilename)
	if err != nil {
		return errors.Wrapf(err, "Failed to create output file %v", outputFilename)
	}
	defer outputFile.Close()
	err = t.Execute(outputFile, &templateData{Report: rpReport})
	if err != nil {
		return errors.Wrap(err, "Failed to run html template")
	}
	return nil
}

type resourceSpecMap = map[string][]string

var supportedResources resourceSpecMap = map[string][]string{
	"iam":            {"role"},
	"glacier":        {"Vault"},
	"efs":            {"FileSystem"},
	"organizations":  nil,
	"kms":            {"Key"},
	"apigateway":     {"RestApi"},
	"ecr":            {"Repository"},
	"es":             {"Domain"},
	"ec2":            {"Images", "Snapshots"},
	"lambda":         {"Alias", "Function", "LayerVersion"},
	"rds":            {"DBSnapshot", "DBClusterSnapshot"},
	"s3":             {"Bucket"},
	"secretsmanager": {"Secret"},
	"ses":            {"Identity"},
	"sns":            {"Topic"},
	"sqs":            {"Queue"},
}

func serviceSpec(r resourceSpecMap) string {
	services := []string{}
	for service, resources := range r {
		if resources == nil {
			services = append(services, service)
		} else {
			services = append(services, service+"="+strings.Join(resources, ","))
		}
	}
	return strings.Join(services, ";")
}

func main() {
	pkger.Include("/templates")
	pkger.Include("/queries")
	var skipIntrospector, leavePostgresUp, reusePostgres, logIntrospector, printToStdOut, skipIntrospectorPull bool
	var outputDir, introspectorRef string
	flag.BoolVar(&skipIntrospector, "skip-introspector", false, "Skip running an import, use existing data")
	flag.BoolVar(&skipIntrospectorPull, "skip-introspector-pull", false, "Skip pulling the introspector docker image. Allows for using a local image")
	flag.StringVar(&introspectorRef, "introspector-ref", "", "Override the introspector docker image to use")
	flag.BoolVar(&leavePostgresUp, "leave-postgres", false, "Leave postgres running in a docker container")
	flag.BoolVar(&reusePostgres, "reuse-postgres", false, "Reuse an existing postgres instance, if it is running")
	flag.BoolVar(&logIntrospector, "log-introspector", false, "Pass through logs from introspector docker image")
	flag.BoolVar(&printToStdOut, "print-to-stdout", false, "Print report results to stdout")
	flag.StringVar(&outputDir, "output", "output", "Specify a directory for output")
	flag.Parse()
	ds, err := ds.NewSession()
	if err != nil {
		panic(errors.Wrap(err, "Failed to get docker client. Is it installed?"))
	}
	importer := &ps.DBCredential{
		Username: "introspector",
		Password: "introspector",
	}
	superuser := &ps.DBCredential{
		Username: "postgres",
		Password: "postgres",
	}
	postgresService, err := ps.NewDockerPostgresService(ds, ps.DockerPostgresOptions{
		ReuseExisting:       reusePostgres,
		SuperUserCredential: superuser,
		ContainerName:       "rpCheckup-db",
	})
	if err != nil {
		panic(err)
	}
	if !skipIntrospector {
		awsCreds, err := loadAwsCredentials(ds.Ctx)
		if err != nil {
			panic(err)
		}
		i, err := introspector.New(ds, postgresService, introspector.Options{
			LogDockerOutput: logIntrospector,
			SkipDockerPull:  skipIntrospectorPull,
			InspectorRef:    introspectorRef,
		})
		if err != nil {
			panic(err)
		}
		spec := serviceSpec(supportedResources)
		log.Infof("Running introspector with service spec %v", spec)
		log.Info("Introspector run may take a few minutes")
		err = i.ImportAWSService(awsCreds, spec)
		if err != nil {
			panic(err)
		}
		err = i.ShutDown()
		if err != nil {
			panic(err)
		}
	}
	report, err := report.Generate(postgresService.ConnectionString(importer))
	if err != nil {
		panic(err)
	}
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		err = os.Mkdir(outputDir, 0755)
		if err != nil {
			panic(err)
		}
	}
	if printToStdOut {
		printReportRows(report.Rows)
	}
	err = writeHTMLReport(report, outputDir+"/index.html")
	if err != nil {
		panic(err)
	}
	err = writeCSVReport(report, outputDir+"/report.csv")
	if err != nil {
		panic(err)
	}
	log.Infof("Reports written to directory %v", outputDir)
	if !leavePostgresUp {
		err = postgresService.ShutDown()
		if err != nil {
			panic(err)
		}
	}
}
