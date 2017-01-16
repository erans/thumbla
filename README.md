# Thumbla - Image serving and manipulation

Written by Eran Sandler ([@erans](https://twitter.com/erans)) &copy; 2017

Thumbla is a quick server to fetch and manipulate photos. It can securely fetch from remote locations that do not have to be publically available.

Supported Fetchers:
- Local (fetch from a local folder on the server)
- HTTP/S (fetch from a remote HTTP/S server)
- AWS S3 Bucket - fetch from an S3 bucket that is not necessarily publically accessible to anonymous users
- Google Storage Bucket - fetch from a Google Storage bucket that is not necessarily publically accessible to anonymous users

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
- Additional security features for the various fetchers
- Face cropping manipulator - crop based on existing detected faces (will support Google Vision and Microsoft Face API)
- Paste - paste another image on top an existing one
- various images enhancmenents protocols
- Recipes - store complex image manipulation recipes and only pass input parameters
