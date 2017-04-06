## Overview

`new_cluster` is a tool that lets me bring up pachyderm clusters quickly.
Ideally, in the long run, it will work with `svp` so that each of my clients
has its own cluster config (including .kubecfg and ADDRESS variable) and I can
work with one cluster inside a given client without disturbing the others.
