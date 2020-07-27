<p align="center">
  <img alt="Voyager" src="https://github.com/crutcha/voyager-probe/blob/readmeupdate/.github/gopher_space.png?raw=true" height="200" />
  <h3 align="center">Voyager Probe</h3>
  <p align="center">Probe agent for <a href="https://github.com/crutcha/voyager-server">Voyager Server</a></p>
  <p align="center">
    <a href="https://github.com/crutcha/voyager-probe/releases/latest"><img alt="GitHub release" src="https://img.shields.io/github/release/crutcha/voyager-probe.svg?logo=github&style=flat-square"></a>
    <img src="https://img.shields.io/github/workflow/status/crutcha/voyager-probe/tests" />
  </p>
</p>

Voyager probe agent is a lightweight metrics collector that emits data back to Voyager server. Configuration for the agent is controlled by Voyager server.

### Env Variables 

The following environment variables are required by the agent for communication back to voyager server.

| Name                 | Type    | Description                                               |
|----------------------|---------|-----------------------------------------------------------|
| `VOYAGER_SERVER`     | String  | HTTPS endpoint of voyager server ie: voyager.mydomain.com |
| `VOYAGER_PROBE_TOKEN`| String  | Auth token generated for probe agent by voyager server    |

### Debugging

Agent can be started with `-d` flag to enable debug logging.
