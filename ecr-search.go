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
	ecrTypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

const (
	maxResults    int32  = 1000 // for ListImages, maximum value
	regexDefault  string = "^latest"
	regionDefault string = "us-east-1"
)

var (
	client *ecr.Client   // aws client
	image  string        // the docker image to search
	images []imageDetail // our results
	regex  string        // the regex to match tags against
	region string        // the aws region to use
)

type imageDetail struct {
	name, date string
}

func init() {
	flag.StringVar(&regex, "regex", regexDefault, "Regex used to filter tags")
	flag.StringVar(&image, "image", "", "The image name to search for")
	flag.StringVar(&region, "region", regionDefault, "The AWS region to use")
	flag.Parse()

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(err)
	}

	client = ecr.NewFromConfig(cfg, func(o *ecr.Options) {
		o.Region = "us-west-2"
	})
}

func sortTags() {
	sort.Slice(images, func(i, j int) bool {
		return images[i].date > images[j].date
	})
}

func buildImageDetails(i []ecrTypes.ImageIdentifier) {
	input := ecr.DescribeImagesInput{
		ImageIds:       i,
		RepositoryName: &image,
	}

	// TODO: add paging @ 100 per request to match DescribeImages
	// maximum ImageIds
	result, err := client.DescribeImages(context.TODO(), &input)
	if err != nil {
		fmt.Println(err)
	}

	for _, image := range result.ImageDetails {
		for i := range image.ImageTags {
			id := imageDetail{
				name: image.ImageTags[i],
				date: image.ImagePushedAt.String(),
			}

			images = append(images, id)
		}
	}
}

func output() {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	for _, tag := range images {
		fmt.Fprintf(w, "%v:%v\t\t%v\n", image, tag.name, tag.date)
	}
	w.Flush()
}

func getAllTags() *ecr.ListImagesOutput {
	filter := &ecrTypes.ListImagesFilter{
		TagStatus: ecrTypes.TagStatusTagged,
	}

	maxResults := maxResults
	input := &ecr.ListImagesInput{
		RepositoryName: aws.String(image),
		MaxResults:     &maxResults,
		Filter:         filter,
	}

	result, err := client.ListImages(context.TODO(), input)
	if err != nil {
		fmt.Println(err)
	}

	return result
}

func getImageTags() []ecrTypes.ImageIdentifier {
	var i []ecrTypes.ImageIdentifier

	for _, tag := range getAllTags().ImageIds {
		matched, _ := regexp.MatchString(regex, *tag.ImageTag)
		if matched {
			i = append(i, tag)
		}
	}

	return i
}

func main() {
	tagList := getImageTags()
	buildImageDetails(tagList)
	sortTags()
	output()
}
