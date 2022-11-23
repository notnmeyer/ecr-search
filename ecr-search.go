package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"text/tabwriter"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

type ECRAPI interface {
	DescribeImages(ctx context.Context, params *ecr.DescribeImagesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error)
	ListImages(ctx context.Context, params *ecr.ListImagesInput, optFns ...func(*ecr.Options)) (*ecr.ListImagesOutput, error)
}

type imageDetail struct {
	name, date string
}

type ecrSearch struct {
	client ECRAPI
	image  string        // the docker image to search
	images []imageDetail // our results
	regex  string        // the regex to match tags against
	region string        // the aws region to use
}

func NewEcrSearch() *ecrSearch {
	return &ecrSearch{}
}

func (search *ecrSearch) sortTags() {
	sort.Slice(search.images, func(i, j int) bool {
		return search.images[i].date > search.images[j].date
	})
}

func (search *ecrSearch) buildImageDetails(i []types.ImageIdentifier) {
	input := ecr.DescribeImagesInput{
		ImageIds:       i,
		RepositoryName: &search.image,
	}

	// TODO: add paging @ 100 per request to match DescribeImages
	// maximum ImageIds
	result, err := search.client.DescribeImages(context.TODO(), &input)
	if err != nil {
		fmt.Println(err)
	}

	for _, image := range result.ImageDetails {
		for i := range image.ImageTags {
			id := imageDetail{
				name: image.ImageTags[i],
				date: image.ImagePushedAt.String(),
			}

			search.images = append(search.images, id)
		}
	}
}

func (search *ecrSearch) print() {
	search.sortTags()
	var (
		minwidth, tabwidth, padding int  = 0, 0, 1
		padchar                     byte = ' '
		flags                       uint = 0
	)
	w := tabwriter.NewWriter(os.Stdout, minwidth, tabwidth, padding, padchar, flags)
	for _, tag := range search.images {
		fmt.Fprintf(w, "%v:%v\t\t%v\n", search.image, tag.name, tag.date)
	}
	w.Flush()
}

func (search *ecrSearch) getAllTags() (result *ecr.ListImagesOutput) {
	var (
		filter     = &types.ListImagesFilter{TagStatus: types.TagStatusTagged}
		maxResults = int32(1000)
	)

	input := &ecr.ListImagesInput{
		RepositoryName: aws.String(search.image),
		Filter:         filter,
		MaxResults:     &maxResults,
	}

	result, err := search.client.ListImages(context.TODO(), input)
	if err != nil {
		panic(err)
	}
	return
}

func (search *ecrSearch) getImageTags() (i []types.ImageIdentifier) {
	for _, tag := range search.getAllTags().ImageIds {
		matched, _ := regexp.MatchString(search.regex, *tag.ImageTag)
		if matched {
			i = append(i, tag)
		}
	}
	return
}

func main() {
	search := NewEcrSearch()

	flag.StringVar(&search.regex, "regex", "^latest", "Regex used to filter tags")
	flag.StringVar(&search.image, "image", "", "The image name to search for")
	flag.StringVar(&search.region, "region", "us-east-1", "The AWS region to use")
	flag.Parse()

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(err)
	}

	search.client = ecr.NewFromConfig(cfg, func(o *ecr.Options) {
		o.Region = search.region
	})

	tagList := search.getImageTags()
	search.buildImageDetails(tagList)
	search.print()
}
