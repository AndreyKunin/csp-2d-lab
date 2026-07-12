import React, {useCallback} from 'react';

interface CheckboxProps {
  id: string;
  checked: boolean;
  label: string;
  onChange: (flag: boolean) => void;
}

export const Checkbox: React.FC<CheckboxProps> = ({ id, checked, label, onChange }) => {

  const handleClick = useCallback((value: boolean) => {
    if (checked !== value) {
      onChange(value);
    }
  }, [checked]);

  return (
      <div className={"radioGroup horizontal"}>
        <div key={id+'_wrapper'} onClick={() => handleClick(!checked)}>
          <div key={id+'_sketch'}></div>
          <div key={id+'_overlay'} className={checked ? 'visible' : 'hidden'}></div>
          <div key={id+'_label'}>{label}</div>
        </div>
      </div>
  );
};
