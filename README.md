# fuzz-mjpeg
##Motion JPEG fuzzer and server written in Go

##Serving - Under Construction

This server streams jpeg frames from a folder of the users' choosing at a regular interval. 
It currently does not have support for continuous data streaming, as it does not monitor the folder
(it enumerates all .jpeg files at time of initial connection)

##Fuzzing - Under Construction

Fuzzing mode can be enabled at process launch. The user can also override the fuzzer's range on certain parameters.


There are currently three parameters being fuzzed:
* Generated per session
  * Boundary string
* Generated per frame
  * Response time (seconds before next frame)
  * Length provided in header
  
## TODO
* Documentation
  * Verbose terminal messages
  * Help command to help
* Fuzzing
  * Recording sessions (through stdout or file)
  * Playback from file (not just the files but also the framerate
* Serving
  * Provide single-file serving mode to push updates.
  * Provide commandline ability to choose port

