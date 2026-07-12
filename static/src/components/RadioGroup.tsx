import React, {useCallback} from 'react';

export type RadioGroupOption = {
  id: string;
  value: string;
};

export type RadioGroupType = 'horizontal' | 'vertical'

interface RadioGroupProps {
  id: string;
  options: RadioGroupOption[];
  selectedId: string;
  type: RadioGroupType;
  onChange: (optionId: string) => void;
  allowDeselect?: boolean;
}

export const RadioGroup: React.FC<RadioGroupProps> = ({ id, options, selectedId, type, onChange, allowDeselect }) => {

  const handleClick = useCallback((optionId: string) => {
    if (selectedId !== optionId) {
      onChange(optionId);
    } else if (allowDeselect) {
      onChange('');
    }
  }, [selectedId, onChange, allowDeselect]);

  return (
      <div className={"radioGroup " + type}>
        {options.map((option, index) => (
          <div key={id+'_'+option.id+'_wrapper'} onClick={() => handleClick(option.id)}>
            <div key={id+'_'+option.id+'_sketch'}></div>
            <div key={id+'_'+option.id+'_overlay'} className={option.id === selectedId ? 'visible' : 'hidden'}></div>
            <div key={id+'_'+option.id+'_label'}>{option.value}</div>
          </div>
        ))}
      </div>
  );
};
