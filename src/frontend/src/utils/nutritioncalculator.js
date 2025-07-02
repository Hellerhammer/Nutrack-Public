import { apiService } from '../services/apiService';

export const calculateNutrition = async (
  weight,
  height,
  age,
  gender,
  activityLevel,
  weeklyWeightChange = 0
) => {
  try {
      // Calculate nutrition using the backend API
    const response = await apiService.makeRequest('POST', '/settings/calculate-nutrients', {
      weight,
      height,
      age,
      gender,
      activityLevel,
      weeklyWeightChange
    });

    return {
      calories: response.calories,
      proteins: response.proteins,
      carbs: response.carbs,
      fat: response.fat,
    };

  } catch (error) {
    console.error('Error calculating nutrition:', error);
    throw error;
  }
};

export const calculateNutrientsFromCaloriesAndWeight = async (calories, weight) => {
  try {
    const response = await apiService.makeRequest('POST', '/settings/calculate-from-calories-and-weight', {
      calories,
      weight
    });
    
    return {
      calories: response.calories,
      proteins: response.proteins,
      carbs: response.carbs,
      fat: response.fat,
    };
  } catch (error) {
    console.error('Error calculating nutrients from calories:', error);
    // Fallback to local calculation if API call fails
    const proteins = weight * 2; // 2g protein per kg body weight
    const fat = (calories * 0.3) / 9; // 30% of calories from fat
    const carbs = (calories - (proteins * 4 + fat * 9)) / 4; // Rest from carbs

    return {
      calories: Math.round(calories),
      proteins: Math.round(proteins),
      carbs: Math.round(carbs),
      fat: Math.round(fat),
    };
  }
};
