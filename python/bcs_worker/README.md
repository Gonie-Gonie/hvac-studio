# bcs-worker

`bcs-worker` is the Python worker package used by HVAC Studio's Go runner to
load and execute user-defined Python components in a structured subprocess
protocol.

Projects normally receive this package through the source tree, portable
package, runtime package, or runtime export. The wheel is built for release
review and for environments that prefer installing the worker package into a
managed Python environment.
