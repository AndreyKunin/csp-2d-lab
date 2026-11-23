Feature: Simulation lifecycle
  Pause, resume, delete and calls on unknown or finished jobs.

  Background:
    Given the server is running with config "fast"

  @FT-LC-01 @FT-LC-02 @FT-LC-03
  Scenario: A paused simulation does not progress and can be resumed
    Given a long-running simulation for fixture "D13-mixed-20"
    And the epoch number increases within 10 seconds
    When I pause the simulation
    Then the response status is 200
    And the response job status is "paused"
    And the response message is "Simulation paused"
    And the job status is "paused"
    And the epoch number does not change for 1 second
    When I resume the simulation
    Then the response status is 200
    And the job status is "running"
    And the epoch number increases within 5 seconds

  @FT-LC-04
  Scenario: Pausing twice is rejected
    Given a long-running simulation for fixture "D13-mixed-20"
    When I pause the simulation
    Then the response status is 200
    When I pause the simulation
    Then the response status is not successful
    And the error message contains "already paused"
    And the job status is "paused"

  @FT-LC-05
  Scenario: Resuming a running simulation is rejected
    Given a long-running simulation for fixture "D13-mixed-20"
    When I resume the simulation
    Then the response status is not successful
    And the error message contains "already running"

  @FT-LC-06
  Scenario: A simulation completes at its epoch limit
    Given a running simulation for fixture "D13-mixed-20" with maxEpochsCount 30
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And the epoch number is at most 30

  @FT-LC-07
  Scenario: Deleting a running simulation
    Given a long-running simulation for fixture "D13-mixed-20"
    When I delete the simulation
    Then the response status is 200
    And the response message is "Simulation stopped and deleted"
    And the simulation no longer exists

  @FT-LC-08
  Scenario: Deleting a paused simulation
    Given a long-running simulation for fixture "D13-mixed-20"
    When I pause the simulation
    And I delete the simulation
    Then the response status is 200
    And the simulation no longer exists

  @FT-LC-09
  Scenario: Deleting a completed simulation
    Given a running simulation for fixture "D13-mixed-20" with maxEpochsCount 20
    And I wait for job completion within 60 seconds
    When I delete the simulation
    Then the response status is 200
    And the response message is "Simulation stopped and deleted"
    And the simulation no longer exists

  @FT-LC-10
  Scenario: Deleting an unknown simulation
    When I call "delete" for an unknown simulation
    Then the response status is not successful
    And the error message contains "not found"

  @FT-LC-11
  Scenario Outline: Calls on an unknown simulation report "not found"
    When I call "<action>" for an unknown simulation
    Then the response status is not successful
    And the error message contains "not found"
    And the response arrives within 2 seconds

    Examples:
      | action    |
      | pause     |
      | resume    |
      | status    |
      | result    |
      | delete    |
      | configure |

  @FT-LC-12
  Scenario: Pausing a completed simulation never hangs
    Given a running simulation for fixture "D13-mixed-20" with maxEpochsCount 5
    And I wait for job completion within 30 seconds
    When I pause the simulation
    Then the response arrives within 2 seconds
    And the response status is not successful
    And the server is healthy

  @FT-LC-13
  Scenario: Resuming a completed simulation never hangs
    Given a running simulation for fixture "D13-mixed-20" with maxEpochsCount 5
    And I wait for job completion within 30 seconds
    When I resume the simulation
    Then the response arrives within 2 seconds
    And the response status is not successful
    And the server is healthy

  @FT-LC-14
  Scenario: Many pause/resume cycles
    Given a running simulation for fixture "D13-mixed-20" with parameters:
      | maxEpochsCount | 3000      |
      | windowSize     | 250       |
      | cycleCount     | 1000000   |
    When I pause and resume the simulation 20 times
    Then every repeated call succeeded
    When I wait for job completion within 120 seconds
    Then the job status is "completed"

  @FT-LC-15
  Scenario: Control calls on a failed simulation never hang
    Given a running simulation for fixture "D10-too-big"
    And I wait for job completion within 20 seconds
    And the job status is "error"
    When I pause the simulation
    Then the response arrives within 2 seconds
    When I resume the simulation
    Then the response arrives within 2 seconds
    And the server is healthy

  @FT-LC-16
  Scenario: Stopping the way the UI does: pause, fetch the result, delete
    Given a long-running simulation for fixture "D13-mixed-20"
    And the epoch number increases within 10 seconds
    When I pause the simulation
    Then the response status is 200
    When I fetch the result
    Then every solution is valid
    When I delete the simulation
    Then the response status is 200
    And the simulation no longer exists
