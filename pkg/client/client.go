package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/Alexamakans/wharf-common-api-client/pkg/apiclient"
	"github.com/Alexamakans/wharf-common-api-client/pkg/remoteprovider"
	"github.com/iver-wharf/wharf-api/pkg/model/database"
)

// Client implements remoteprovider.Client.
type Client struct {
	remoteprovider.BaseClient
}

func NewClient(ctx context.Context, token, apiURLPrefix, remoteProviderURL string) *Client {
	return &Client{
		*remoteprovider.NewClient(ctx, token, apiURLPrefix, remoteProviderURL),
	}
}

func (c *Client) FetchFile(projectIdentifier remoteprovider.ProjectIdentifier, fileName string) ([]byte, error) {
	orgName, remoteProjectID := projectIdentifier.Values[0], projectIdentifier.Values[1]
	path := fmt.Sprintf("%s/_apis/git/repositories/%s/items", orgName, remoteProjectID)
	return apiclient.DoGetBytes(c, c.RemoteProviderURL, path, "scopePath", fmt.Sprintf("/%s", ".wharf-ci.yml"))
}

func (c *Client) FetchBranches(projectIdentifier remoteprovider.ProjectIdentifier) ([]remoteprovider.WharfBranch, error) {
	orgName, remoteProjectID := projectIdentifier.Values[0], projectIdentifier.Values[1]
	path := fmt.Sprintf("%s/_apis/git/repositories/%s/refs", orgName, remoteProjectID)

	const refBranchesFilter = "heads/"
	const refBranchesPrefix = "refs/" + refBranchesFilter

	var projectRefs struct {
		Value []struct {
			ObjectID string `json:"objectId"`
			Name     string `json:"name"`
			Creator  struct {
				ID          string `json:"id"`
				DisplayName string `json:"displayName"`
				URL         string `json:"url"`
				UniqueName  string `json:"uniqueName"`
				ImageURL    string `json:"imageUrl"`
				Descriptor  string `json:"descriptor"`
			} `json:"creator"`
			URL string `json:"url"`
		} `json:"value"`
		Count int `json:"count"`
	}

	err := apiclient.DoGetUnmarshal(&projectRefs, c, c.RemoteProviderURL, path, "api-version", "5.0", "filter", refBranchesFilter)
	if err != nil {
		return []remoteprovider.WharfBranch{}, fmt.Errorf("failed getting branches for project with remote id %q in organization %s: %w", remoteProjectID, orgName, err)
	}

	branches := make([]remoteprovider.WharfBranch, len(projectRefs.Value))
	for _, ref := range projectRefs.Value {
		name := strings.TrimPrefix(ref.Name, refBranchesPrefix)
		branches = append(branches, remoteprovider.WharfBranch{
			Name: name,
		})
	}

	return branches, nil
}

func (c *Client) FetchProjectByGroupAndProjectName(groupName, projectName string) (remoteprovider.WharfProject, error) {
	orgName, projName := splitStringOnceRune(groupName, '/')
	repoName := projectName

	path := fmt.Sprintf("%s/%s/_apis/git/repositories/%s", orgName, projName, repoName)
	fmt.Printf("path=%s\n", path)

	var repo repository
	err := apiclient.DoGetUnmarshal(&repo, c, c.RemoteProviderURL, path, "api-version", "5.0")
	if err != nil {
		return remoteprovider.WharfProject{}, fmt.Errorf("failed getting project named %s in %s: %w", projName, groupName, err)
	}

	project := remoteprovider.WharfProject{
		Project: database.Project{
			Name:        repo.Name,
			GroupName:   fmt.Sprintf("%s/%s", orgName, repo.Project.Name),
			Description: repo.Project.Description,
			GitURL:      repo.SSHURL,
		},
		RemoteProjectID: repo.ID}

	return project, nil
}

func (c *Client) WharfProjectToIdentifier(project remoteprovider.WharfProject) remoteprovider.ProjectIdentifier {
	orgName, _ := splitStringOnceRune(project.GroupName, '/')
	return remoteprovider.ProjectIdentifier{
		Values: []string{orgName, project.RemoteProjectID},
	}
}

// copied from github.com/iver-wharf/wharf-provider-azuredevops
func splitStringOnceRune(value string, delimiter rune) (a, b string) {
	const notFoundIndex = -1
	delimiterIndex := strings.IndexRune(value, delimiter)
	if delimiterIndex == notFoundIndex {
		a = value
		b = ""
		return
	}
	a = value[:delimiterIndex]
	b = value[delimiterIndex+1:] // +1 to skip the delimiter
	return
}

type repository struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	URL              string  `json:"url"`
	Project          project `json:"project"`
	DefaultBranchRef string  `json:"defaultBranch"`
	Size             int64   `json:"size"`
	RemoteURL        string  `json:"remoteUrl"`
	SSHURL           string  `json:"sshUrl"`
}

type project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	State       string `json:"state"`
	Revision    int64  `json:"revision"`
	Visibility  string `json:"visibility"`
}
