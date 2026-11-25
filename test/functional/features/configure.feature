Feature: Runtime reconfiguration
  Changing simulation parameters of an existing job.

  Configure replaces the whole parameter set with the defaults plus the
  values sent, so long-running scenarios always resend maxEpochsCount,
  windowSize and cycleCount to keep the job running.

  Background:
    Given the server is running with config "fast"

  @FT-CFG-01
  Scenario: Configuring a running job
    Given a long-running simulation for fixture "D13-mixed-20"
    When I configure the simulation with:
      | mutationRate   | 0.02      |
      | maxEpochsCount | 100000000 |
      | windowSize     | 250       |
      | cycleCount     | 1000000   |
    Then the response status is 200
    And the response message is "Simulation configured"
    And the reported parameter "mutationRate" is 0.02
    And the job status is "running"
    And the epoch number increases within 5 seconds

  @FT-CFG-02
  Scenario: Configuring a paused job keeps it paused
    Given a long-running simulation for fixture "D13-mixed-20"
    And I pause the simulation
    When I configure the simulation with:
      | mutationRate   | 0.04      |
      | maxEpochsCount | 100000000 |
      | windowSize     | 250       |
      | cycleCount     | 1000000   |
    Then the response status is 200
    And the job status is "paused"
    And the reported parameter "mutationRate" is 0.04

  @FT-CFG-03
  Scenario: Lowering the epoch limit below the current epoch ends the job
    Given a long-running simulation for fixture "D13-mixed-20"
    And the epoch number increases within 10 seconds
    And I pause the simulation
    And I remember the current epoch
    When I configure maxEpochsCount to the remembered epoch
    Then the response status is 200
    When I resume the simulation
    And I wait for job completion within 5 seconds
    Then the job status is "completed"
    And the epoch number is at most the remembered epoch plus 1

  @FT-CFG-04
  Scenario: Raising the epoch limit of a running job
    Given a running simulation for fixture "D13-mixed-20" with parameters:
      | maxEpochsCount | 3000      |
      | windowSize     | 250       |
      | cycleCount     | 1000000   |
    When I configure the simulation with:
      | maxEpochsCount | 100000000 |
      | windowSize     | 250       |
      | cycleCount     | 1000000   |
    Then the response status is 200
    And the epoch number exceeds 3000 within 60 seconds

  @FT-CFG-05
  Scenario Outline: Configure without populationSize keeps the population size
    Given a running simulation for fixture "D13-mixed-20" with parameters:
      | populationSize.min | <size>    |
      | populationSize.max | <size>    |
      | maxEpochsCount     | 100000000 |
      | windowSize         | 250       |
      | cycleCount         | 1000000   |
    When I configure the simulation with:
      | mutationRate   | 0.05      |
      | maxEpochsCount | 100000000 |
      | windowSize     | 250       |
      | cycleCount     | 1000000   |
    Then the response status is 200
    And the reported parameter "populationSize" is <size>
    And the epoch number increases within 5 seconds
    And the job status is "running"

    # The default-derived size for D13 in the "fast" profile is 60.
    Examples:
      | size |
      | 30   |
      | 100  |

  @FT-CFG-06
  Scenario: Changing the elite count changes the elite set
    Given a long-running simulation for fixture "D13-mixed-20"
    When I configure the simulation with:
      | eliteCount     | 3         |
      | maxEpochsCount | 100000000 |
      | windowSize     | 250       |
      | cycleCount     | 1000000   |
    Then the reported parameter "eliteCount" is 3
    And the selected species count becomes 3 within 5 seconds

  @FT-CFG-07
  Scenario: Shrinking the window size
    Given a long-running simulation for fixture "D13-mixed-20"
    And the epoch number increases within 10 seconds
    When I configure the simulation with:
      | windowSize     | 5         |
      | maxEpochsCount | 100000000 |
    And I poll the status for 1 second
    Then the job status is not "error"
    And all statistics are finite

  # Growing the window after shrinking it crashes the job: the window slice is
  # shorter than the new size, and addEpoch indexes past its end once the
  # epoch number exceeds the new size.
  @FT-CFG-07 @pending
  Scenario: Growing the window size after the run has passed it
    Given a running simulation for fixture "D13-mixed-20" with parameters:
      | windowSize     | 5         |
      | maxEpochsCount | 100000000 |
    And the epoch number exceeds 60 within 10 seconds
    When I configure the simulation with:
      | windowSize     | 50        |
      | maxEpochsCount | 100000000 |
    And I poll the status for 1 second
    Then the job status is not "error"

  # Validated before the job is paused (BE-02): a rejected request leaves the
  # job running with its parameters. eliteCount 1000 is above the population
  # size (60 for D13 in the fast profile).
  @FT-CFG-08
  Scenario Outline: Invalid parameter values are rejected and change nothing
    Given a long-running simulation for fixture "D13-mixed-20"
    When I configure the simulation with:
      | <key>          | <value>   |
      | maxEpochsCount | 100000000 |
      | windowSize     | 250       |
      | cycleCount     | 1000000   |
    Then the response status is 400
    And the job status is "running"
    And the reported parameter "eliteCount" is 5

    Examples:
      | key          | value |
      | eliteCount   | 0     |
      | eliteCount   | 1000  |
      | mutationRate | -0.1  |
      | mutationRate | 1.5   |

  @FT-CFG-08
  Scenario: A window size of 0 is rejected
    Given a long-running simulation for fixture "D13-mixed-20"
    When I configure the simulation with:
      | windowSize     | 0         |
      | maxEpochsCount | 100000000 |
    Then the response status is 400
    And the job status is "running"

  @FT-CFG-09
  Scenario: Configuring a completed job never hangs
    Given a running simulation for fixture "D13-mixed-20" with maxEpochsCount 5
    And I wait for job completion within 30 seconds
    When I configure the simulation with:
      | mutationRate | 0.05 |
    Then the response arrives within 2 seconds
    And the response status is 409
    And the server is healthy

  @FT-CFG-10
  Scenario: Malformed configuration JSON leaves the job running
    Given a long-running simulation for fixture "D13-mixed-20"
    When I send the raw action body '{"action": "configure", "parameters": {"mutationRate": '
    Then the response status is 400
    And the job status is "running"
