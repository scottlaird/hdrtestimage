# hdrtestimage

An image for testing HDR, along with Go code for generating it.

This bit of Go code generates a 16-bit TIFF file that contains a test
image that covers a very wide range of brightness and color values,
and is intended to be used for testing HDR image conversion pipelines.

This image is currently written out as a TIFF file with no color
profile attached due to limitations in Go's image writing code.  It is
intended to be Rec. 2100 PQ W203, and I've included an AVIF file
generated via Photoshop using the TIFF as a source.  I'll replace this
with a correctly-generated file once I find a tool that can do it in
Go.

![test image](hdrtestimage.avif)
