# Thumbla - Micro service for service and manipulating images

Written by Eran Sandler ([@erans](https://twitter.com/erans)) http://eran.sandler.co.il &copy; 2017

Thumbla is a micro service that fetches and manipulates images. It can securely fetch from remote locations that are not publicly available.

Supported Fetchers:
- Local - fetch from a local directory on the server (the directory can also be mounted from a remote location and shared across servers)
- HTTP/S - fetch from a remote HTTP/S server
- AWS S3 Bucket - fetch from an S3 bucket. Supports accessing a private S3 bucket.
- Google Storage Bucket - fetch from a Google Storage bucket. Support accessing a private Google Storage bucket.

Fetched images can then be manipulated via manipulators such as:
- Resize (proportionally or not)
- Fit (fit the image to a specified size proportionally)
- Crop
- Flip Horizontally
- Flip Vertically
- Rotate
- Shear Horizontally
- Shear Vertically

What's still missing:
- Additional security features for the various fetchers (support auth for HTTP/S requests)
- Allow configuring sub paths that will go to different image repositories (i.e. /aaa/ goes to an S3 bucket, /bbb/ goes to a Google bucket and /ccc/ goes to a remote HTTP server)
- Face cropping manipulator - crop based on existing detected faces (will support Google Vision and Microsoft Face API)
- Paste - paste another image on top an existing one
- various images enhancements (brightness, contrast, levels adjustments etc)
- Recipes - store complex image manipulation recipes and only pass input parameters
