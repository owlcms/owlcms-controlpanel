# Replays

Note: This is an optional module.

#####

This module installs the **Replays** application for OWLCMS jury video review (see [documentation](https://jflamy.github.io/owlcms4/#/JuryReplays)).
- Replays, by default, will read the multicast video feeds published by the **Cameras** module using multicasting. This allows the Replays module to be on a different machine than the Cameras
- The legacy config.toml file is still honored to read cameras directly if you turn off the multicasting option.

#####

Configuration notes:
- You need to check that the camera ports published by the Cameras module match your camera placement

