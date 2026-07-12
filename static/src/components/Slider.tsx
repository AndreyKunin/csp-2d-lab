import React, {useCallback, useEffect, useMemo, useState} from 'react';
import {Model} from '../utils/model';

interface SliderProps {
  model: Model | null;
  selectedSolution: number | null;
  windowMaxSize: number;
  onSelectSheet: (sheet: number) => void;
}

export const Slider: React.FC<SliderProps> = ({ model, selectedSolution, windowMaxSize, onSelectSheet }) => {
  const [selectedSheet, setSelectedSheet] = useState<number | null>(null);
  const [firstSheet, setFirstSheet] = useState<number>(0);
  const solution = useMemo(
      () => selectedSolution !== null ? model?.solutions?.[selectedSolution] : null,
      [model, selectedSolution]
  );
  const sheetCount = useMemo(
      () => solution !== null ? solution?.sheets?.length || 0 : 0,
      [solution]
  );
  const windowSize = useMemo(
      () => Math.min(windowMaxSize, sheetCount),
      [windowMaxSize, sheetCount]
  );

  const selectSheet = useCallback((sheet: number) => {
    setSelectedSheet(sheet);
    onSelectSheet(sheet);
  }, [onSelectSheet])

  const shiftSlider = useCallback((condition: boolean, shift: number) => {
    if (condition) {
      if (selectedSheet !== null) {
        if (selectedSheet < firstSheet + shift) {
          const sheetNumber = firstSheet + shift;
          selectSheet(sheetNumber);
        }
        if (selectedSheet >= firstSheet + windowSize + shift) {
          const sheetNumber = firstSheet + windowSize + shift - 1;
          selectSheet(sheetNumber);
        }
      }
      setFirstSheet(firstSheet + shift);
    }
  }, [firstSheet, selectedSheet, windowSize]);

  useEffect(() => {
    if (selectedSheet == null && sheetCount > 0) {
      selectSheet(0);
    }
  }, [selectedSheet, selectSheet]);


  if (sheetCount === 0) {
    return <div id="slider"></div>;
  }

  return (
      <div className="pagination">
        <div>
          <span
              className={firstSheet > 0 ? 'available' : ''}
              onClick={() => shiftSlider(firstSheet > 0, -1)}
          >
            &lt;
          </span>
        </div>
        <div>
          {Array.from({ length: windowSize }, (_, index) => (
              <div key={`sheet_${firstSheet + index}`}>
                <a
                    className={selectedSheet === firstSheet + index ? 'selectedPage' : ''}
                    onClick={() => selectSheet(firstSheet + index)}
                >
                  Sheet {firstSheet + index + 1}
                </a>
                {index < windowSize - 1 && <span>.</span>}
              </div>
          ))}
        </div>
        <div>
          <span
              className={sheetCount > firstSheet + windowSize ? 'available' : ''}
              onClick={() => shiftSlider(sheetCount > firstSheet + windowSize, 1)}
          >
            &gt;
          </span>
        </div>
      </div>
  );
};

export default Slider;