# TTS-Helper

Very small helper program because I was too stubborn to go back to VSCode to use the Tabletop Simulator extension.

This will help in development of TTSLua scripts via listening to TTS messages on an incoming TCP connection.

As well as sending data to TTS over another outgoing TCP connection.

Features so far:

- Get object data (.lua and .xml files) when TTS communicates that a new game was loaded and when a new object is "scriptified"
- Small REST API server to simulate IDE communication with TTS
  - This works by making the IDE send a curl request to the program so in turn this can send data to TTS

### Notes
I don't expect to use this anywhere else, so probably code quality won't be a priority.
Perchance.