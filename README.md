# Thumbla - Micro service for fetching & manipulating images

Written by Eran Sandler ([@erans](https://twitter.com/erans)) http://eran.sandler.co.il &copy; 2017

Thumbla is a micro service that fetches and manipulates images. It can securely fetch from remote locations that are not publicly available.

## Supported Fetchers:
- **Local** - fetch from a local directory on the server (the directory can also be mounted from a remote location and shared across servers)
- **HTTP/S** - fetch from a remote HTTP/S server
- **AWS S3** - fetch from an S3 bucket. Supports accessing a private S3 bucket.
- **Google Storage** - fetch from a Google Storage bucket. Support accessing a private Google Storage bucket.

## Supported Manipulators
Fetched images can then be manipulated via manipulators such as:
- **Resize** - resize the image proportionally or not
- **Fit** - fit the image to a specified size proportionally
- **Crop** - crop parts of the images ()
- **Flip Horizontally**
- **Flip Vertically**
- **Rotate** - rotate the image. resize the image to include the complete rotated original image
- **Shear Horizontally**
- **Shear Vertically**

## What's still missing:
- Additional security features for the various fetchers (support auth for HTTP/S requests)
- Face cropping manipulator - crop based on existing detected faces (will support Google Vision and Microsoft Face API)
- Paste - paste another image on top an existing one
- various images enhancements (brightness, contrast, levels adjustments etc)
- Recipes - store complex image manipulation recipes and only pass input parameters
- HTTP fetchers restrictions are configurable but are not yet active
- Crop manipulator doesn't parses "minus" values (to relative to the width or height) and percent values
