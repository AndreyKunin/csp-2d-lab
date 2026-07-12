import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import { Results } from './components/Results';
import { Visualizer } from './components/Visualizer';
import {Model, Solution} from './utils/model';
import {SimulationResults, startSimulation} from './services/requests';
import {InputForm, InputFormData} from "./components/InputForm";
import {Simulation} from "./utils/simulation";
import {ErrorList} from "./components/ErrorList";
import {Dashboard} from "./components/lab/Dashboard";

const App: React.FC = () => {
  const resultsRef = useRef<HTMLDivElement>(null);
  const [errors, setErrors] = useState<string[]>([]);
  const [selectedSolution, setSelectedSolution] = useState<number | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [model, setModel] = useState<Model | null>(null);
  const [simulation, setSimulation] = useState<Simulation | null>(null);
  const [solutions, setSolutions] = useState<Solution[] | null>(null);
  const [dashboardKey, setDashboardKey] = useState(0);

  const reset = useCallback(() => {
    setErrors([]);
    setSelectedSolution(null);
    setModel(null);
    setSimulation(null);
    setSolutions(null);
  }, []);

  const onInputFormSubmit = useCallback(async (formData: InputFormData) => {
    setErrors([]);
    const model = new Model(formData);
    const simulation = new Simulation(formData);
    const modelValidationErrors = model.validate();
    const parameterValidationErrors = simulation.validate();
    const validationErrors = [...modelValidationErrors, ...parameterValidationErrors];
    if (validationErrors.length > 0) {
      setErrors(validationErrors);
      return;
    }
    setIsLoading(true);
    setSolutions(null);
    setSelectedSolution(null);
    try {
      const response = await startSimulation(model, simulation.initialParameters);
      if (response.error) {
        setErrors([response.error]);
      } else {
        response.enrich(simulation);
        simulation.startTime = new Date();
        setModel(model);
        setSimulation(simulation);
        setDashboardKey(prev => prev + 1);
      }
    } catch (error) {
      setErrors(['Failed to connect to server. Please try again.']);
    } finally {
      setIsLoading(false);
    }
  }, []);


  const onInputFormError = useCallback(async (messages: string[]) => {
    setErrors(messages);
  }, []);

  const onRuntimeError = useCallback(async (error: unknown) => {
    function errorToString(error: unknown): string {
      if (error instanceof Error) {
        return error.message;
      }
      if (typeof error === 'string') {
        return error;
      }
      try {
        return JSON.stringify(error);
      } catch {
        return String(error);
      }
    }
    setErrors([errorToString(error)]);
  }, []);

  const onResultsHandler = useCallback((results: SimulationResults) => {
    if (model) {
      model.solutions = results.solutions;
    }
    setSolutions(results.solutions);
  }, [model]);

  const handleSolutionSelect = useCallback((index: number) => {
    setSelectedSolution(index);
  }, [model]);

  const displayErrors = useMemo(
      () => errors && errors.length > 0,
      [errors]
  );
  const displayDashboard = useMemo(
      () => simulation?.id && simulation.status,
      [simulation]
  );
  const displayResults = useMemo(
      () => solutions && solutions.length > 0,
      [solutions]
  );

  useEffect(() => {
    if (solutions && solutions.length > 0 && resultsRef.current) {
      resultsRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [solutions]);

  return (
      <div className="layout">
        <div className="inputForm">
          <InputForm
              onSubmit={onInputFormSubmit}
              onError={onInputFormError}
              onReset={reset}
              isLoading={isLoading}
          />
        </div>
        {displayErrors && (
            <ErrorList errors={errors} />
        )}
        {displayDashboard && (
            <Dashboard
                key={dashboardKey}
                simulationId={simulation!.id!}
                initialStatus={simulation!.status!}
                initialLabMode={!!(simulation!.initialParameters) || false}
                startTime={simulation?.startTime}
                hasSheets={!!model?.sheets}
                units={model ? model.units : ''}
                onError={onRuntimeError}
                onResultsHandler={onResultsHandler}
            />
        )}
        {displayResults && (
            <div className="results" ref={resultsRef}>
              <Results
                  solutions={solutions!}
                  units={model ? model.units : ''}
                  selectedSolution={selectedSolution}
                  onSolutionSelect={handleSolutionSelect}
              />
              <Visualizer
                  model={model}
                  selectedSolution={selectedSolution}
                  setErrors={setErrors}
              />
            </div>
        )}
      </div>
  );
};

export default App;