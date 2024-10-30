# Thumbla - Micro service for fetching & manipulating images

Written by Eran Sandler ([@erans](https://twitter.com/erans)) http://eran.sandler.co.il &copy; 2018

![Thumbla](examples/img/thumbla-logo.png)

Thumbla is a micro service that fetches and manipulates images. It can securely fetch from remote locations that are not publicly available such as storage buckets and manipulate images in multiple ways.

![badge](https://github.com/erans/thumbla/actions/workflows/docker-image.yml/badge.svg)

## Supported Read Image Formats:
- **JPEG**
- **PNG**
- **WEBP**
- **GIF** (read-only, single frame)
- **SVG**

## Supported Write Image Formats:
- **JPEG**
- **PNG**
- **WEBP**

### Handling SVG
While SVGs do contain some sizing information they are vector formats and can be scaled to any size.
In order for Thumbla to correctly handle SVG files we need to rasterize it, basically converting it into a PNG, in order
to further process it with all the manipulators available.

To do that you can specify a size for the SVG that Thumbla will rasterize into by using this format:
`https://example.com/i/pics/subpath_inside_bucket%2Fmyfile.svg|{W},{H}/output:f=jpg`

Where `{W}` or `{H}` are the width and height pixel size of the raserized version of the SVG file.
To perform relative scaling based on either width or height, send -1 to the unknown variable.
For example, creating a scaled rasterized version of 300px width, send:
`https://example.com/i/pics/subpath_inside_bucket%2Fmyfile.svg|300,-1/output:f=jpg`

Without specifying a size, we will try to use the deafult SVG size which, in most cases, is small.

## Supported Fetchers:
- **Local** - fetch from a local directory on the server (the directory can also be mounted from a remote location and shared across servers)
- **HTTP/S** - fetch from a remote HTTP/S server
- **[AWS S3](https://aws.amazon.com/s3/)** - fetch from an S3 bucket. Supports accessing a private S3 bucket that is not accessible to the world.
- **[Google Storage](https://cloud.google.com/storage/)** - fetch from a Google Storage bucket. Support accessing a private Google Storage bucket that is not accessible to the world.
- **[Azure Blob Storage](https://azure.microsoft.com/en-us/services/storage/blobs/)** - fetch from an Azure Blob Storage bucket. Support accessing a private Azure Blob Storage bucket that is not accessible to the world.
- **[DigitalOcean Spaces](https://www.digitalocean.com/products/spaces/)** - fetch from a DigitalOcean Spaces bucket. Support accessing a private DigitalOcean Spaces bucket that is not accessible to the world.
- **[Cloudflare R2](https://www.cloudflare.com/r2/)** - fetch from a Cloudflare R2 bucket. Support accessing a private Cloudflare R2 bucket that is not accessible to the world.

With AWS S3 and Google Storage support you can allow access to only a specific folder within a private bucket.

### Fetchers Configuration
- **Local** - fetches files from a locally accessible folder on the server
  - **path** - the path to the location where images resize. This path will serve as the root path and all images will be referenced relative to it. This path can also be remotely mounted and acecssed by multiple servers as Thumbla only needs read access.


- **HTTP/S** - fetches files from HTTP/S URLs
  - **userName** - use if the HTTP URL requires a username
  - **password** - use if the HTTP URL requires a password
  - **secure** - fetch only from HTTPS sources
  - **restrictHosts** - an array of restricted hostname this instance of the fetcher will retrieve images from. For example, if the restrictHosts is set to `images.example.com` only URLs that has this hostname will be fetches, others will be rejected.
  - **restrictPaths** - an array of restricted paths that this instance of the fetcher will retrieve image from from. For example, if  the restrictPaths is set to `/img/` only URLs with that path will be fetched, others will be rejected.

  - **restrictHosts** and **restrictPaths** can be combined to restrict a certain host and a certain path, for example.


- **AWS S3** - fetches files from an AWS S3 bucket
  - **region** - the AWS S3 bucket region
  - **accessKeyID** - the access Key ID to access this bucket. Not needed if you are using IAM roles
  - **secretAccessKey** - the access key secret. Not needed if you are using IAM roles

  - To fetch from S3 buckets use URLs in the format of:
  `http://s3-aws-region.amazonaws.com/bucket/path/file`


- **Google Storage** - fetches files from a Google Storage bucket
  - **bucket** - bucket Name
  - **path** - will be used as a root path. If set you can use relataive paths when fetching images to manipulate
  - **projectId** - the project ID that is associated with that Google Storage bucket
  - **securitySource** - can be `background` if the machine running Thumbla has access to that (or all buckets), otherwise set to `file`
  - **serviceAccountJSONFile** - a path to the service account JSON file that will allow access to the specified bucket. Only needed when *securitySource* is set to `file`

- **Azure Blob Storage** - fetches files from an Azure Blob Storage bucket
  - **accountName** - the Azure Blob Storage account name
  - **accountKey** - the Azure Blob Storage account key

- **DigitalOcean Spaces** - fetches files from a DigitalOcean Spaces bucket
  - **accessID** - the DigitalOcean Spaces access ID
  - **secretKey** - the DigitalOcean Spaces secret key

- **Cloudflare R2** - fetches files from a Cloudflare R2 bucket
  - **accessKey** - the Cloudflare R2 access key
  - **secretKey** - the Cloudflare R2 secret key

## Supported Manipulators
Fetched images can then be manipulated via manipulators such as:
- **Resize** - resize the image proportionally or not
- **Fit** - fit the image to a specified size proportionally
- **Crop** - crop parts of the images
- **Flip Horizontally** - flips the image horizontally
- **Flip Vertically** - flips the image vertically
- **Rotate** - rotate the image. resize the image to include the complete rotated original image
- **Shear Horizontally**
- **Shear Vertically**
- **Face Crop**
- **Paste** - allows pasting (preferably PNG) images (initial support)
- **brightness** - adjust the brightness of the image
- **contrast** - adjust the contrast of the image

## Face Cropping
Face cropping crops an image based on the faces visible in it while keeping the original image aspect ratio. Humans recognize and react to faces much more than any other objects. The face crop manipulator is a great way to generate thumbnails or focused images that will mostly show the faces in the picture.

Supported Facial Detection APIs:
  - [AWS Rekognition](https://aws.amazon.com/rekognition/)
  - [Google Vision API](https://cloud.google.com/vision/) (using the facial detection features)
  - [Azure Face API](https://azure.microsoft.com/en-us/services/cognitive-services/face/)

Here is an example of how Face cropping detects faces (blue rectangles) and how it would crop these images (the yellow rectangle):<br/>

![Debugging Face Cropping](examples/img/facecrop-debug.jpg)

The result would look like this:<br/>
![Result Face Cropping](examples/img/facecrop-result.jpg)

## Usage - Configuration
See [`config-example.yml`](config-example.yml) for an example of the configuration.

The configuration file has 2 major sections:
- `fetchers` where different sources of the images are defined. This will define all the necessary parameters needed to access a certain source. For example, credentials of accessing a bucket in the cloud, as well as various restrictions on fetching files from HTTP/S source.

- `paths` are the paths that will be accessible from this server. You can mix and match paths with fetches, allow a single server to fetch sources from different cloud services. Paths always end with a `/` (slash) as Thumbla automatically adds additional parts to it to be able to process requests.

## URL Structure
Let's assume we have configured a path called `/i/pics/` using an AWS S3 fetcher. This will allow us to fetch files from a certain location inside an S3 bucket. Fetching the files would look like this:
`https://example.com/i/pics/subpath_inside_bucket%2Fmyfile.jpg/output:f=jpg`

`subpath_inside_bucket%2Fmyfile.jpg` is the URL encoded relative path inside the bucket (unencoded it looks like this: subpath_inside_bucket/myfile.jpg). If you are using an HTTP fetcher this can also be a full blown URL as long as it is URL encoded.

If we want to resize this image the URL would look like this:
`https://example.com/i/pics/subpath_inside_bucket%2Fmyfile.jpg/resize:w=350/output:f=jpg`

We can mix and match different manipulators, for example:
`https://example.com/i/pics/subpath_inside_bucket%2Fmyfile.jpg/rotate:a=35/resize:w=350/output:f=jpg`

This will rotate the image 35 degrees and resize the result to a width of 350px keep the image aspect ratio and calculating the matching height.

## Running Under Kubernetes
- The best way to run the mico service under Kubernetes with custom configuration is to update the configuration file as a configmap:
```
kubectl create configmap thumbla-config --from-file=thumbla.yml
```

You can then mount `thumbla-config` as a volume inside your container and point to it using an environment varaible `THUMBLACFG`. For example:
```
apiVersion: v1
kind: ReplicationController
metadata:
  name: thumbla
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: thumbla
    spec:
      containers:
      - name: thumbla
        image: erans/thumbla:latest
        volumeMounts:
        -
          name: config-volume
          mountPath: /etc/config
        env:
          -
            name: THUMBLACFG
            value: "/etc/config/thumbla.yml"
          -
            name: PORT
            value: "8000"
        ports:
        - containerPort: 8000

      volumes:
        - name: config-volume
          configMap:
            name: thumbla-config
```

The above configuration will mount `thumbla-config` map onto `/etc/config` inside the container. The environment variable `THUMBLACFG` points to the config file `thumbla.yml` under `/etc/config` (the mounted volume).


## What's still missing:
- various images enhancements
- Recipes - store complex image manipulation recipes and only pass input parameters

