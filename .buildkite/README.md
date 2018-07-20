# BuildKite Build Files
We have pre-populated your buildkite build files with a standard template. Please feel free to update and edit this directory. We have tried to have the files themselves be as descriptive as possible.

If you want to make changes to what steps are run, please start with .buildkite/pipeline.yml, and it will lead you on from there.

One important difference from Jenkins; is that your build nodes are stateless now. A step might run on one node; and the next step might run on a completely different node. This makes managing your environment a little trickier than it used to be. You'll need to set / get data that you need across multiple steps (which will wind up being treated as environment variables), and maybe even use the pre-command hook inside of your hooks directory.

If you need a file later on, you will need to archive it, and then download the file in any step that needs that file.

For example; our automated versioning requires a file called version_wf. In our global pre-command; we download the value of our vesion, and write that out into a file every time a command is run.

For more information on meta-data, check out this link: https://buildkite.com/docs/agent/cli-meta-data

Another point of interest; is that any steps that do not have a wait between them, will run in parallel. Do you have a bunch of tests that you want to run? Separate them out of your .jenkins-docker file, and add them to the buildkite pipeline file with out waits, and you will speed up your build significantly.
