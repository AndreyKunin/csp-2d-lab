import React from 'react';

interface UpButtonProps {
    className?: string;
}

export const UpButton: React.FC<UpButtonProps> = ({className}) => {
    const handleUp = async () => {
        window.scrollTo({
            top: 0,
            left: 0,
            behavior: 'smooth'
        });
    };

    return (
        <div onClick={handleUp} className={className}>Up</div>
    );
};

export default UpButton;