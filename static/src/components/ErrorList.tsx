import React from 'react';

interface ErrorListProps {
    errors: string[];
}

export const ErrorList: React.FC<ErrorListProps> = ({ errors }) => {
    return (
        <div className="error itemsList">
            <ul>
                {errors.map((error, index) => (
                    <li key={index}>{error}</li>
                ))}
            </ul>
        </div>
    );
};
