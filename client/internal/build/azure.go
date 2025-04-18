// Copyright 2025 R5 Labs
// This file is part of the R5 Core library.
//
// This software is provided "as is", without warranty of any kind,
// express or implied, including but not limited to the warranties
// of merchantability, fitness for a particular purpose and
// noninfringement. In no event shall the authors or copyright
// holders be liable for any claim, damages, or other liability,
// whether in an action of contract, tort or otherwise, arising
// from, out of or in connection with the software or the use or
// other dealings in the software.

package build

import (
	"context"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

// AzureBlobstoreConfig is an authentication and configuration struct containing
// the data needed by the Azure SDK to interact with a specific container in the
// blobstore.
type AzureBlobstoreConfig struct {
	Account   string // Account name to authorize API requests with
	Token     string // Access token for the above account
	Container string // Blob container to upload files into
}

// AzureBlobstoreUpload uploads a local file to the Azure Blob Storage. Note, this
// method assumes a max file size of 64MB (Azure limitation). Larger files will
// need a multi API call approach implemented.
//
// See: https://msdn.microsoft.com/en-us/library/azure/dd179451.aspx#Anchor_3
func AzureBlobstoreUpload(path string, name string, config AzureBlobstoreConfig) error {
	if *DryRunFlag {
		fmt.Printf("would upload %q to %s/%s/%s\n", path, config.Account, config.Container, name)
		return nil
	}
	// Create an authenticated client against the Azure cloud
	credential, err := azblob.NewSharedKeyCredential(config.Account, config.Token)
	if err != nil {
		return err
	}
	u := fmt.Sprintf("https://%s.blob.core.windows.net/%s", config.Account, config.Container)
	container, err := azblob.NewContainerClientWithSharedKey(u, credential, nil)
	if err != nil {
		return err
	}
	// Stream the file to upload into the designated blobstore container
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()

	blockblob := container.NewBlockBlobClient(name)
	_, err = blockblob.Upload(context.Background(), in, nil)
	return err
}

// AzureBlobstoreList lists all the files contained within an azure blobstore.
func AzureBlobstoreList(config AzureBlobstoreConfig) ([]*azblob.BlobItemInternal, error) {
	// Create an authenticated client against the Azure cloud
	credential, err := azblob.NewSharedKeyCredential(config.Account, config.Token)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("https://%s.blob.core.windows.net/%s", config.Account, config.Container)
	container, err := azblob.NewContainerClientWithSharedKey(u, credential, nil)
	if err != nil {
		return nil, err
	}
	var maxResults int32 = 5000
	pager := container.ListBlobsFlat(&azblob.ContainerListBlobFlatSegmentOptions{
		Maxresults: &maxResults,
	})
	var allBlobs []*azblob.BlobItemInternal
	for pager.NextPage(context.Background()) {
		res := pager.PageResponse()
		allBlobs = append(allBlobs, res.ContainerListBlobFlatSegmentResult.Segment.BlobItems...)
	}
	return allBlobs, pager.Err()
}

// AzureBlobstoreDelete iterates over a list of files to delete and removes them
// from the blobstore.
func AzureBlobstoreDelete(config AzureBlobstoreConfig, blobs []*azblob.BlobItemInternal) error {
	if *DryRunFlag {
		for _, blob := range blobs {
			fmt.Printf("would delete %s (%s) from %s/%s\n", *blob.Name, blob.Properties.LastModified, config.Account, config.Container)
		}
		return nil
	}
	// Create an authenticated client against the Azure cloud
	credential, err := azblob.NewSharedKeyCredential(config.Account, config.Token)
	if err != nil {
		return err
	}
	u := fmt.Sprintf("https://%s.blob.core.windows.net/%s", config.Account, config.Container)
	container, err := azblob.NewContainerClientWithSharedKey(u, credential, nil)
	if err != nil {
		return err
	}
	// Iterate over the blobs and delete them
	for _, blob := range blobs {
		blockblob := container.NewBlockBlobClient(*blob.Name)
		if _, err := blockblob.Delete(context.Background(), &azblob.DeleteBlobOptions{}); err != nil {
			return err
		}
		fmt.Printf("deleted  %s (%s)\n", *blob.Name, blob.Properties.LastModified)
	}
	return nil
}
